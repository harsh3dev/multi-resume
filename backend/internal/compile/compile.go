// Package compile renders untrusted LaTeX to PDF.
//
// Layers: Compiler (this file) adds caching, asset fetching and PDF storage on
// top of a Renderer. The Renderer is either LocalRenderer (latexmk in this
// process: bubblewrap for local dev, DirectRunner in the compiler service) or
// RemoteRenderer (HTTP to the compiler service, so the main service carries no
// TeX toolchain).
//
// Security model (BUILD-PLAN §7.6/§11, tech-stack §6):
//   - compiler name allowlisted before it becomes a CLI flag
//   - shell-escape attempts (\write18, --shell-escape, …) rejected upfront;
//     latexmk also runs with -no-shell-escape and openin_any/openout_any=p
//   - per-job fresh temp dir; asset paths validated against traversal
//   - 15 s timeout, prlimit memory cap, concurrency semaphore, no network
//   - clients only ever see the parsed error line — never paths or full logs
package compile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	// MaxTexBytes bounds one uploaded .tex source.
	MaxTexBytes = 2 << 20
	// MaxAssets bounds how many asset files one compile may reference.
	MaxAssets = 64
	// MaxAssetBytes bounds one asset file.
	MaxAssetBytes = 10 << 20
)

// Result of a successful compile.
type Result struct {
	PDFKey    string
	PageCount int
	// Log is reserved for client-safe log excerpts; empty on success.
	Log    string
	Cached bool
}

// CompileError is a client-safe compile failure. Message contains the parsed
// LaTeX error line (paths stripped) or a generic reason.
type CompileError struct {
	Message string
}

func (e *CompileError) Error() string { return e.Message }

// Compiler compiles .tex sources with caching and per-draft cancellation.
// It fetches asset bytes and stores PDFs; the actual rendering is delegated to
// a Renderer.
type Compiler struct {
	cache    PDFCache
	store    ObjectStore
	renderer Renderer
	pdf      PDFTools

	mu      sync.Mutex
	seq     uint64
	cancels map[string]cancelSlot
}

type cancelSlot struct {
	token  uint64
	cancel context.CancelFunc
}

// Options configures a Compiler. Cache and Store are required, plus either a
// Renderer or a Runner (from which a LocalRenderer is built). PDFTools
// defaults to PopplerTools.
type Options struct {
	Cache    PDFCache
	Store    ObjectStore
	Renderer Renderer
	// Runner builds a LocalRenderer when Renderer is nil.
	Runner         Runner
	PDFTools       PDFTools
	MaxParallel    int
	Timeout        time.Duration
	MaxMemoryBytes int64
}

func New(opts Options) (*Compiler, error) {
	if opts.Cache == nil || opts.Store == nil || (opts.Renderer == nil && opts.Runner == nil) {
		return nil, errors.New("compile: Cache, Store, and a Renderer or Runner are required")
	}
	if opts.PDFTools == nil {
		opts.PDFTools = PopplerTools{}
	}
	renderer := opts.Renderer
	if renderer == nil {
		local, err := NewLocalRenderer(LocalOptions{
			Runner: opts.Runner, PDFTools: opts.PDFTools, MaxParallel: opts.MaxParallel,
			Timeout: opts.Timeout, MaxMemoryBytes: opts.MaxMemoryBytes,
		})
		if err != nil {
			return nil, err
		}
		renderer = local
	}
	return &Compiler{
		cache:    opts.Cache,
		store:    opts.Store,
		renderer: renderer,
		pdf:      opts.PDFTools,
		cancels:  make(map[string]cancelSlot),
	}, nil
}

// Compile renders tex to a PDF. assets maps tex-relative path → object-store
// key. compiler may be "" to auto-detect from the preamble.
func (c *Compiler) Compile(ctx context.Context, tex string, assets map[string]string, compiler string) (*Result, error) {
	if compiler == "" {
		compiler = DetectCompiler(preambleOf(tex))
	}
	if !IsValidCompiler(compiler) {
		return nil, fmt.Errorf("compile: unsupported compiler %q", compiler)
	}
	if len(tex) > MaxTexBytes {
		return nil, fmt.Errorf("compile: source too large")
	}
	// Fail closed on shell-escape attempts before anything touches disk.
	if ContainsShellEscape(tex) {
		return nil, &CompileError{Message: "shell escape is not allowed"}
	}
	if len(assets) > MaxAssets {
		return nil, fmt.Errorf("compile: too many assets")
	}

	fetched := make(map[string][]byte, len(assets))
	hashes := make(map[string]string, len(assets))
	for texPath, objKey := range assets {
		if err := ValidateAssetPath(texPath); err != nil {
			return nil, fmt.Errorf("compile: %w", err)
		}
		var data []byte
		if objKey == PlaceholderKey {
			data = PlaceholderImage(texPath)
		} else {
			var err error
			data, err = c.store.Get(ctx, objKey)
			if err != nil {
				return nil, fmt.Errorf("compile: fetch asset: %w", err)
			}
		}
		if len(data) > MaxAssetBytes {
			return nil, fmt.Errorf("compile: asset too large")
		}
		sum := sha256.Sum256(data)
		hashes[texPath] = hex.EncodeToString(sum[:])
		fetched[texPath] = data
	}

	key := CacheKey(tex, hashes, compiler)
	if entry, ok, err := c.cache.Get(ctx, key); err == nil && ok {
		return &Result{PDFKey: entry.PDFKey, PageCount: entry.PageCount, Cached: true}, nil
	}

	rendered, err := c.renderer.Render(ctx, RenderRequest{Tex: tex, Compiler: compiler, Assets: fetched})
	if err != nil {
		return nil, err
	}
	pdfKey := "pdf/" + key + ".pdf"
	if err := c.store.Put(ctx, pdfKey, rendered.PDF); err != nil {
		return nil, fmt.Errorf("compile: store pdf: %w", err)
	}
	// A failed cache upsert must not fail the compile — worst case we
	// recompile next time.
	_ = c.cache.Put(ctx, CacheEntry{Key: key, PDFKey: pdfKey, PageCount: rendered.PageCount})
	return &Result{PDFKey: pdfKey, PageCount: rendered.PageCount}, nil
}

// CompileDraft is Compile with per-draft cancellation: a new compile for the
// same draft cancels the older one, whose context cancellation kills latexmk.
func (c *Compiler) CompileDraft(ctx context.Context, draftID, tex string, assets map[string]string, compiler string) (*Result, error) {
	if draftID == "" {
		return c.Compile(ctx, tex, assets, compiler)
	}
	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	if prev, ok := c.cancels[draftID]; ok {
		prev.cancel()
	}
	c.seq++
	slot := cancelSlot{token: c.seq, cancel: cancel}
	c.cancels[draftID] = slot
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if cur, ok := c.cancels[draftID]; ok && cur.token == slot.token {
			delete(c.cancels, draftID)
		}
		c.mu.Unlock()
		cancel()
	}()
	return c.Compile(ctx, tex, assets, compiler)
}

// PageCount reports the number of pages in a produced PDF (pdfinfo).
func (c *Compiler) PageCount(pdfPath string) (int, error) {
	return c.pdf.PageCount(pdfPath)
}

// ExtractText extracts the text of a produced PDF (pdftotext), used by the
// probe-compile check.
func (c *Compiler) ExtractText(pdfPath string) (string, error) {
	return c.pdf.ExtractText(pdfPath)
}
