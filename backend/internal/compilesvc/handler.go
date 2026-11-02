// Package compilesvc is the HTTP face of the compiler service: it accepts one
// compile request, renders it, and returns the PDF. It holds no secrets and no
// state — everything it needs arrives in the request.
package compilesvc

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/harsh3dev/multi-resume/backend/internal/compile"
)

// Config configures the handler.
type Config struct {
	Renderer compile.Renderer
	// Token, when set, must be presented as "Authorization: Bearer <token>".
	// When empty the handler does no authentication of its own and the
	// platform (Cloud Run IAM) must; callers choose that explicitly.
	Token string
	// Recycle, when set, is called after a response if the instance should be
	// replaced: a job timed out or failed unexpectedly (its process tree is
	// suspect), or RecycleAfterJobs was reached. Cloud Run starts a fresh one.
	Recycle          func()
	RecycleAfterJobs int64
}

const (
	maxFieldBytes   = 64
	maxTexFieldSize = compile.MaxTexBytes + 1
)

type handler struct {
	cfg  Config
	jobs atomic.Int64
}

// New returns the service router.
func New(cfg Config) http.Handler {
	h := &handler{cfg: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST "+compile.CompilePath, h.compile)
	return mux
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (h *handler) authorized(r *http.Request) bool {
	if h.cfg.Token == "" {
		return true
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(h.cfg.Token)) == 1
}

func (h *handler) compile(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	req, err := readRequest(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}

	res, err := h.cfg.Renderer.Render(r.Context(), *req)
	h.afterJob(err)
	var (
		ce  *compile.CompileError
		inv *compile.InvalidRequestError
	)
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set(compile.HeaderPageCount, strconv.Itoa(res.PageCount))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(res.PDF)
	case errors.As(err, &ce):
		writeErr(w, http.StatusUnprocessableEntity, ce.Message)
	case errors.As(err, &inv):
		writeErr(w, http.StatusBadRequest, "invalid request")
	case r.Context().Err() != nil:
		// The caller went away; nothing to send.
	default:
		// Never log the source or assets — only that a job failed.
		slog.Error("render failed", "error", err.Error())
		writeErr(w, http.StatusInternalServerError, "internal error")
	}
}

// afterJob decides whether this instance should be replaced.
func (h *handler) afterJob(err error) {
	if h.cfg.Recycle == nil {
		return
	}
	n := h.jobs.Add(1)
	var ce *compile.CompileError
	timedOut := errors.As(err, &ce) && ce.Message == compile.MsgTimeout
	unexpected := err != nil && !timedOut && !errors.As(err, &ce) && !isInvalid(err)
	if timedOut || unexpected || (h.cfg.RecycleAfterJobs > 0 && n >= h.cfg.RecycleAfterJobs) {
		go h.cfg.Recycle()
	}
}

func isInvalid(err error) bool {
	var inv *compile.InvalidRequestError
	return errors.As(err, &inv)
}

// readRequest parses the multipart body with hard limits: whole-body cap,
// per-field caps, known field names only, no duplicates.
func readRequest(w http.ResponseWriter, r *http.Request) (*compile.RenderRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, compile.MaxRequestBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}
	req := &compile.RenderRequest{Assets: map[string][]byte{}}
	var haveTex, haveCompiler bool
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if err := readPart(part, req, &haveTex, &haveCompiler); err != nil {
			part.Close()
			return nil, err
		}
		part.Close()
	}
	if !haveTex || !haveCompiler {
		return nil, errors.New("missing field")
	}
	return req, nil
}

func readPart(part *multipart.Part, req *compile.RenderRequest, haveTex, haveCompiler *bool) error {
	name := part.FormName()
	switch {
	case name == "compiler":
		if *haveCompiler {
			return errors.New("duplicate field")
		}
		b, err := readLimited(part, maxFieldBytes)
		if err != nil {
			return err
		}
		req.Compiler, *haveCompiler = string(b), true
	case name == "tex":
		if *haveTex {
			return errors.New("duplicate field")
		}
		b, err := readLimited(part, maxTexFieldSize)
		if err != nil {
			return err
		}
		req.Tex, *haveTex = string(b), true
	case strings.HasPrefix(name, compile.AssetFieldPrefix):
		p := strings.TrimPrefix(name, compile.AssetFieldPrefix)
		if _, dup := req.Assets[p]; dup || len(req.Assets) >= compile.MaxAssets {
			return errors.New("bad asset")
		}
		b, err := readLimited(part, compile.MaxAssetBytes+1)
		if err != nil {
			return err
		}
		req.Assets[p] = b
	default:
		return errors.New("unknown field")
	}
	return nil
}

// readLimited reads at most max bytes and fails if the part is longer.
func readLimited(r io.Reader, max int) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, int64(max)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > max {
		return nil, errors.New("field too large")
	}
	return b, nil
}
