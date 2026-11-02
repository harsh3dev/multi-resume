package compile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/semaphore"
)

const (
	// MaxTotalAssetBytes bounds all asset bytes of one compile. It keeps a
	// remote request under Cloud Run's 32 MB body limit.
	MaxTotalAssetBytes = 20 << 20
	// MaxPDFBytes bounds a produced PDF.
	MaxPDFBytes = 20 << 20
	// maxImageDimension / maxImagePixels bound decoded image size (pixel bombs).
	maxImageDimension = 8000
	maxImagePixels    = 50_000_000
)

// MsgTimeout is the client-safe message for a compile that ran out of time.
const MsgTimeout = "compilation timed out"

// RenderRequest is everything one compile needs, as plain bytes. It is the
// wire contract between the main service and the compiler service.
type RenderRequest struct {
	Tex      string
	Compiler string
	// Assets maps tex-relative path → file bytes.
	Assets map[string][]byte
}

// RenderResult is a produced PDF.
type RenderResult struct {
	PDF       []byte
	PageCount int
}

// Renderer turns LaTeX source into a PDF. It is a pure function of its input:
// no storage, no cache. LocalRenderer runs latexmk in this process;
// RemoteRenderer calls the compiler service.
//
// Failures are *CompileError (client-safe, shown to the user),
// *InvalidRequestError (the request itself was rejected), or any other error
// (infrastructure; never shown to users).
type Renderer interface {
	Render(ctx context.Context, req RenderRequest) (*RenderResult, error)
}

// InvalidRequestError means the request broke a limit or rule before any
// compile started. It is not the user's LaTeX at fault.
type InvalidRequestError struct{ Reason string }

func (e *InvalidRequestError) Error() string { return "compile: " + e.Reason }

func invalid(format string, args ...any) error {
	return &InvalidRequestError{Reason: fmt.Sprintf(format, args...)}
}

// ValidateRenderRequest enforces every input limit. Both the compiler service
// and LocalRenderer call it, so limits hold even if a caller skips its own
// checks. Shell-escape attempts come back as a client-safe *CompileError.
func ValidateRenderRequest(req RenderRequest) error {
	if !IsValidCompiler(req.Compiler) {
		return invalid("unsupported compiler %q", req.Compiler)
	}
	if len(req.Tex) > MaxTexBytes {
		return invalid("source too large")
	}
	// Fail closed on shell-escape attempts before anything touches disk.
	if ContainsShellEscape(req.Tex) {
		return &CompileError{Message: "shell escape is not allowed"}
	}
	if len(req.Assets) > MaxAssets {
		return invalid("too many assets")
	}
	total := 0
	for p, data := range req.Assets {
		if err := ValidateAssetPath(p); err != nil {
			return invalid("%v", err)
		}
		switch strings.ToLower(path.Ext(p)) {
		case ".png", ".jpg", ".jpeg":
		default:
			return invalid("unsupported asset type")
		}
		if len(data) > MaxAssetBytes {
			return invalid("asset too large")
		}
		total += len(data)
		if total > MaxTotalAssetBytes {
			return invalid("assets too large")
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width < 1 || cfg.Height < 1 ||
			cfg.Width > maxImageDimension || cfg.Height > maxImageDimension ||
			cfg.Width*cfg.Height > maxImagePixels {
			return invalid("unreadable or oversized image")
		}
	}
	return nil
}

// LocalRenderer runs latexmk through a Runner (bubblewrap locally, the
// hardened direct runner in the compiler service), with a concurrency limit.
type LocalRenderer struct {
	sem     *semaphore.Weighted
	runner  Runner
	pdf     PDFTools
	timeout time.Duration
	mem     int64
	// disabled compilers are refused before a job starts.
	disabled map[string]bool
}

// LocalOptions configures a LocalRenderer. Runner is required.
type LocalOptions struct {
	Runner         Runner
	PDFTools       PDFTools // default PopplerTools
	MaxParallel    int      // default 4
	Timeout        time.Duration
	MaxMemoryBytes int64
	// DisabledCompilers are rejected up front (e.g. lualatex, which runs
	// arbitrary Lua, where there is no sandbox).
	DisabledCompilers []string
}

func NewLocalRenderer(o LocalOptions) (*LocalRenderer, error) {
	if o.Runner == nil {
		return nil, errors.New("compile: Runner is required")
	}
	if o.MaxParallel <= 0 {
		o.MaxParallel = 4
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.MaxMemoryBytes <= 0 {
		o.MaxMemoryBytes = DefaultMaxMemoryBytes
	}
	if o.PDFTools == nil {
		o.PDFTools = PopplerTools{}
	}
	dis := make(map[string]bool, len(o.DisabledCompilers))
	for _, c := range o.DisabledCompilers {
		dis[c] = true
	}
	return &LocalRenderer{
		sem: semaphore.NewWeighted(int64(o.MaxParallel)), runner: o.Runner, pdf: o.PDFTools,
		timeout: o.Timeout, mem: o.MaxMemoryBytes, disabled: dis,
	}, nil
}

func (r *LocalRenderer) Render(ctx context.Context, req RenderRequest) (*RenderResult, error) {
	if err := ValidateRenderRequest(req); err != nil {
		return nil, err
	}
	if r.disabled[req.Compiler] {
		return nil, &CompileError{Message: req.Compiler + " is not supported on this server; use pdflatex or xelatex"}
	}
	if err := r.sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer r.sem.Release(1)

	files := make([]assetFile, 0, len(req.Assets))
	for p, data := range req.Assets {
		files = append(files, assetFile{path: p, data: data})
	}
	dir, err := writeJobFiles(req.Tex, files)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	runErr := r.runner.Run(ctx, Job{
		Dir:            dir,
		Compiler:       req.Compiler,
		Timeout:        r.timeout,
		MaxMemoryBytes: r.mem,
	})
	pdfPath := filepath.Join(dir, "main.pdf")
	if runErr != nil {
		if errors.Is(runErr, ErrTimeout) {
			return nil, &CompileError{Message: MsgTimeout}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		msg := "compilation failed"
		if logBytes, err := os.ReadFile(filepath.Join(dir, "main.log")); err == nil {
			if parsed := ParseLogError(string(logBytes), dir); parsed != "" {
				msg = parsed
			}
		}
		return nil, &CompileError{Message: msg}
	}

	if st, err := os.Stat(pdfPath); err != nil || st.Size() == 0 {
		return nil, &CompileError{Message: "compilation produced no PDF"}
	} else if st.Size() > MaxPDFBytes {
		return nil, &CompileError{Message: "the produced PDF is too large"}
	}
	pdf, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, &CompileError{Message: "compilation produced no PDF"}
	}
	pages, err := r.pdf.PageCount(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("compile: page count: %w", err)
	}
	return &RenderResult{PDF: pdf, PageCount: pages}, nil
}
