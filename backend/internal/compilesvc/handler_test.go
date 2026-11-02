package compilesvc

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/harsh3dev/multi-resume/backend/internal/compile"
)

type fakeRenderer struct {
	calls atomic.Int32
	last  compile.RenderRequest
	res   *compile.RenderResult
	err   error
}

func (f *fakeRenderer) Render(_ context.Context, req compile.RenderRequest) (*compile.RenderResult, error) {
	f.calls.Add(1)
	f.last = req
	if f.err != nil {
		return nil, f.err
	}
	if f.res != nil {
		return f.res, nil
	}
	return &compile.RenderResult{PDF: []byte("%PDF-1.5 fake"), PageCount: 2}, nil
}

func multipartBody(t *testing.T, fields map[string]string, files map[string][]byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for k, v := range files {
		p, err := w.CreateFormFile(k, "f")
		if err != nil {
			t.Fatal(err)
		}
		p.Write(v)
	}
	w.Close()
	return &buf, w.FormDataContentType()
}

func post(t *testing.T, h http.Handler, token string, fields map[string]string, files map[string][]byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, fields, files)
	req := httptest.NewRequest(http.MethodPost, compile.CompilePath, body)
	req.Header.Set("Content-Type", ct)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

var okFields = map[string]string{"compiler": "pdflatex", "tex": `\documentclass{article}`}

func TestSuccessReturnsPDFAndPageCount(t *testing.T) {
	fr := &fakeRenderer{}
	rec := post(t, New(Config{Renderer: fr}), "", okFields, map[string][]byte{"asset:img/a.png": []byte("png")})
	if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), "%PDF-") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Page-Count") != "2" || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("headers: %v", rec.Header())
	}
	if string(fr.last.Assets["img/a.png"]) != "png" || fr.last.Compiler != "pdflatex" {
		t.Fatalf("request not passed through: %+v", fr.last)
	}
}

func TestBearerAuthRequiredWhenTokenSet(t *testing.T) {
	fr := &fakeRenderer{}
	h := New(Config{Renderer: fr, Token: "s3cret"})
	for _, tok := range []string{"", "wrong", "s3cret "} {
		if rec := post(t, h, tok, okFields, nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("token %q: got %d, want 401", tok, rec.Code)
		}
	}
	if fr.calls.Load() != 0 {
		t.Fatal("rendered without auth")
	}
	if rec := post(t, h, "s3cret", okFields, nil); rec.Code != 200 {
		t.Fatalf("valid token: %d", rec.Code)
	}
}

func TestMalformedRequestsRejected(t *testing.T) {
	h := New(Config{Renderer: &fakeRenderer{}})
	cases := map[string]struct {
		fields map[string]string
		files  map[string][]byte
	}{
		"missing tex":      {map[string]string{"compiler": "pdflatex"}, nil},
		"missing compiler": {map[string]string{"tex": "x"}, nil},
		"unknown field":    {map[string]string{"compiler": "pdflatex", "tex": "x", "evil": "1"}, nil},
		"oversize tex":     {map[string]string{"compiler": "pdflatex", "tex": strings.Repeat("a", compile.MaxTexBytes+2)}, nil},
	}
	for name, tc := range cases {
		if rec := post(t, h, "", tc.fields, tc.files); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", name, rec.Code)
		}
	}
	// Not multipart at all.
	req := httptest.NewRequest(http.MethodPost, compile.CompilePath, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("json body: got %d", rec.Code)
	}
}

func TestErrorMapping(t *testing.T) {
	cases := map[string]struct {
		err  error
		code int
	}{
		"latex error": {&compile.CompileError{Message: "Undefined control sequence"}, 422},
		"invalid":     {&compile.InvalidRequestError{Reason: "too many assets"}, 400},
		"internal":    {errors.New("dial tcp 10.0.0.1: secret detail"), 500},
	}
	for name, tc := range cases {
		rec := post(t, New(Config{Renderer: &fakeRenderer{err: tc.err}}), "", okFields, nil)
		if rec.Code != tc.code {
			t.Errorf("%s: got %d, want %d", name, rec.Code, tc.code)
		}
		if name == "internal" && strings.Contains(rec.Body.String(), "secret detail") {
			t.Error("internal error detail leaked to the caller")
		}
	}
}

func TestRecycleOnTimeoutAndUnexpectedNotOnLatexError(t *testing.T) {
	check := func(err error, wantRecycle bool) {
		t.Helper()
		var called atomic.Bool
		h := New(Config{Renderer: &fakeRenderer{err: err}, Recycle: func() { called.Store(true) }})
		post(t, h, "", okFields, nil)
		time.Sleep(20 * time.Millisecond)
		if called.Load() != wantRecycle {
			t.Errorf("err=%v: recycle=%v, want %v", err, called.Load(), wantRecycle)
		}
	}
	check(&compile.CompileError{Message: compile.MsgTimeout}, true)
	check(errors.New("boom"), true)
	check(&compile.CompileError{Message: "Undefined control sequence"}, false)
	check(nil, false)
}

func TestRecycleAfterNJobs(t *testing.T) {
	var called atomic.Bool
	h := New(Config{Renderer: &fakeRenderer{}, Recycle: func() { called.Store(true) }, RecycleAfterJobs: 2})
	post(t, h, "", okFields, nil)
	time.Sleep(10 * time.Millisecond)
	if called.Load() {
		t.Fatal("recycled too early")
	}
	post(t, h, "", okFields, nil)
	time.Sleep(10 * time.Millisecond)
	if !called.Load() {
		t.Fatal("did not recycle after N jobs")
	}
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	New(Config{Renderer: &fakeRenderer{}, Token: "x"}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != 200 {
		t.Fatalf("healthz %d", rec.Code)
	}
}

// TestRemoteRendererRoundTrip drives the real client against the real handler
// with a real LocalRenderer + fake runner, end to end over HTTP.
func TestRemoteRendererRoundTrip(t *testing.T) {
	fr := &fakeRenderer{}
	srv := httptest.NewServer(New(Config{Renderer: fr, Token: "tok"}))
	defer srv.Close()
	client, err := compile.NewRemoteRenderer(srv.URL, compile.StaticToken("tok"), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.Render(context.Background(), compile.RenderRequest{
		Tex: `\documentclass{article}`, Compiler: "xelatex",
		Assets: map[string][]byte{"img/p.png": []byte("png-bytes")},
	})
	if err != nil || res.PageCount != 2 || !strings.HasPrefix(string(res.PDF), "%PDF-") {
		t.Fatalf("render: %+v %v", res, err)
	}
	if fr.last.Compiler != "xelatex" || string(fr.last.Assets["img/p.png"]) != "png-bytes" {
		t.Fatalf("server saw %+v", fr.last)
	}

	// LaTeX error → client-safe CompileError.
	fr.err = &compile.CompileError{Message: "Missing $ inserted"}
	_, err = client.Render(context.Background(), compile.RenderRequest{Tex: "x", Compiler: "pdflatex"})
	var ce *compile.CompileError
	if !errors.As(err, &ce) || ce.Message != "Missing $ inserted" {
		t.Fatalf("want CompileError, got %v", err)
	}

	// Infrastructure failure → generic error, no server detail.
	fr.err = errors.New("db password is hunter2")
	_, err = client.Render(context.Background(), compile.RenderRequest{Tex: "x", Compiler: "pdflatex"})
	if err == nil || errors.As(err, &ce) || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("want generic error, got %v", err)
	}

	// Wrong token is not a compile error either.
	bad, _ := compile.NewRemoteRenderer(srv.URL, compile.StaticToken("nope"), 5*time.Second)
	if _, err := bad.Render(context.Background(), compile.RenderRequest{Tex: "x", Compiler: "pdflatex"}); err == nil || errors.As(err, &ce) {
		t.Fatalf("bad token: %v", err)
	}
}
