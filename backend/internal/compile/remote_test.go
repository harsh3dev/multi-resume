package compile

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRemoteRendererURLRules(t *testing.T) {
	tok := StaticToken("t")
	for _, u := range []string{"http://compiler.example.com", "ftp://x", "not a url", "", "https://"} {
		if _, err := NewRemoteRenderer(u, tok, time.Second); err == nil {
			t.Errorf("accepted %q", u)
		}
	}
	for _, u := range []string{"https://compiler-abc.a.run.app", "http://localhost:8081", "http://127.0.0.1:8081"} {
		if _, err := NewRemoteRenderer(u, tok, time.Second); err != nil {
			t.Errorf("rejected %q: %v", u, err)
		}
	}
	if _, err := NewRemoteRenderer("https://x.example.com", nil, time.Second); err == nil {
		t.Error("nil auth accepted")
	}
}

func remoteFor(t *testing.T, h http.HandlerFunc) *RemoteRenderer {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	r, err := NewRemoteRenderer(srv.URL, StaticToken("t"), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	r.retryDelay = time.Millisecond
	return r
}

func renderOnce(r *RemoteRenderer) (*RenderResult, error) {
	return r.Render(context.Background(), RenderRequest{Tex: "x", Compiler: "pdflatex"})
}

func TestRemoteRejectsBadServiceResponses(t *testing.T) {
	bad := map[string]http.HandlerFunc{
		"not a pdf": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(HeaderPageCount, "1")
			w.Write([]byte("<html>"))
		},
		"missing pages": func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("%PDF-1.5")) },
		"absurd pages": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(HeaderPageCount, "99999")
			w.Write([]byte("%PDF-1.5"))
		},
		"oversize": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(HeaderPageCount, "1")
			w.Write([]byte("%PDF-"))
			w.Write(make([]byte, MaxPDFBytes+10))
		},
	}
	for name, h := range bad {
		if _, err := renderOnce(remoteFor(t, h)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRemoteRetriesOnceOnBusyThenSucceeds(t *testing.T) {
	var n atomic.Int32
	r := remoteFor(t, func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set(HeaderPageCount, "1")
		w.Write([]byte("%PDF-1.5"))
	})
	if res, err := renderOnce(r); err != nil || res.PageCount != 1 || n.Load() != 2 {
		t.Fatalf("res=%+v err=%v calls=%d", res, err, n.Load())
	}
}

func TestRemoteDoesNotRetryCompileErrorsOrClientErrors(t *testing.T) {
	for code, want := range map[int]bool{422: true, 400: false, 500: false} {
		var n atomic.Int32
		r := remoteFor(t, func(w http.ResponseWriter, _ *http.Request) {
			n.Add(1)
			w.WriteHeader(code)
			w.Write([]byte(`{"error":"Undefined control sequence"}`))
		})
		_, err := renderOnce(r)
		var ce *CompileError
		if errors.As(err, &ce) != want || n.Load() != 1 {
			t.Errorf("status %d: compileErr=%v calls=%d err=%v", code, errors.As(err, &ce), n.Load(), err)
		}
	}
}

func TestRemoteCancelReturnsContextError(t *testing.T) {
	r := remoteFor(t, func(w http.ResponseWriter, req *http.Request) {
		// Drain the body so the server notices the client disconnecting.
		_, _ = io.Copy(io.Discard, req.Body)
		select {
		case <-req.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	if _, err := r.Render(ctx, RenderRequest{Tex: "x", Compiler: "pdflatex"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestRemoteSendsBearerAndLongCompileErrorsAreCapped(t *testing.T) {
	r := remoteFor(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer t" {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(422)
		w.Write([]byte(`{"error":"` + strings.Repeat("a", 2000) + `"}`))
	})
	_, err := renderOnce(r)
	var ce *CompileError
	if !errors.As(err, &ce) || len(ce.Message) > 500 {
		t.Fatalf("got %v", err)
	}
}

func TestObjectCacheRoundTripAndMisses(t *testing.T) {
	ctx := context.Background()
	c := NewObjectCache(NewMemoryStore())
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Fatal("empty cache hit")
	}
	if err := c.Put(ctx, CacheEntry{Key: "k", PDFKey: "pdf/k.pdf", PageCount: 2}); err != nil {
		t.Fatal(err)
	}
	e, ok, _ := c.Get(ctx, "k")
	if !ok || e.PDFKey != "pdf/k.pdf" || e.PageCount != 2 {
		t.Fatalf("got %+v %v", e, ok)
	}
	// A tampered sidecar pointing elsewhere is a miss, not a redirect.
	_ = c.Store.Put(ctx, "pdf/k.meta.json", []byte(`{"pdfKey":"resume/other.png","pageCount":1}`))
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Fatal("accepted a sidecar pointing outside pdf/<key>.pdf")
	}
}

func TestValidateRenderRequestLimits(t *testing.T) {
	png := PlaceholderImage("a.png")
	ok := RenderRequest{Tex: "x", Compiler: "pdflatex", Assets: map[string][]byte{"a.png": png}}
	if err := ValidateRenderRequest(ok); err != nil {
		t.Fatal(err)
	}
	bad := map[string]RenderRequest{
		"compiler":     {Tex: "x", Compiler: "pdflatex; id"},
		"shell escape": {Tex: `\write18{id}`, Compiler: "pdflatex"},
		"asset type":   {Tex: "x", Compiler: "pdflatex", Assets: map[string][]byte{"a.pdf": png}},
		"traversal":    {Tex: "x", Compiler: "pdflatex", Assets: map[string][]byte{"../a.png": png}},
		"not an image": {Tex: "x", Compiler: "pdflatex", Assets: map[string][]byte{"a.png": []byte("MZ-not-a-png")}},
	}
	for name, req := range bad {
		if err := ValidateRenderRequest(req); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	var ce *CompileError
	if err := ValidateRenderRequest(bad["shell escape"]); !errors.As(err, &ce) {
		t.Errorf("shell escape should be a client-safe CompileError, got %v", err)
	}
}

func TestLocalRendererRefusesDisabledCompiler(t *testing.T) {
	r, err := NewLocalRenderer(LocalOptions{Runner: &fakeRunner{}, PDFTools: fakePDF{}, DisabledCompilers: []string{CompilerLuaLatex}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Render(context.Background(), RenderRequest{Tex: "x", Compiler: CompilerLuaLatex})
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("want CompileError, got %v", err)
	}
}
