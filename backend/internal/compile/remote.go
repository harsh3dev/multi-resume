package compile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Wire contract of the compiler service (POST /v1/compile, multipart):
//
//	compiler          form field, e.g. "pdflatex"
//	tex               form field, the source
//	asset:<path>      one file part per asset, <path> relative to the tex
//
// Success: 200, body = PDF bytes, header X-Page-Count.
// Compile failure: 422 {"error": "<client-safe message>"}.
// Rejected request: 400. Anything else: infrastructure failure.
const (
	CompilePath      = "/v1/compile"
	HeaderPageCount  = "X-Page-Count"
	AssetFieldPrefix = "asset:"
	// MaxRequestBytes bounds one whole compile request body.
	MaxRequestBytes = 25 << 20
	// MaxPages bounds the page count accepted from the service.
	MaxPages = 200
)

// TokenSource supplies the Authorization bearer for compiler requests.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// StaticToken is a shared-secret bearer (non-GCP hosts).
type StaticToken string

func (t StaticToken) Token(context.Context) (string, error) { return string(t), nil }

// MetadataIDToken mints a Google-signed ID token for Audience from the Cloud
// Run metadata server, for calling another IAM-protected Cloud Run service.
type MetadataIDToken struct {
	Audience string
	HTTP     *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

const metadataIdentityURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity"

func (m *MetadataIDToken) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token != "" && time.Until(m.expires) > 5*time.Minute {
		return m.token, nil
	}
	q := url.Values{"audience": {m.Audience}, "format": {"full"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataIdentityURL+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	client := m.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("id token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("id token: status %d", resp.StatusCode)
	}
	m.token = strings.TrimSpace(string(body))
	// ID tokens last an hour; refresh well before that.
	m.expires = time.Now().Add(50 * time.Minute)
	return m.token, nil
}

// RemoteRenderer sends compiles to the compiler service.
type RemoteRenderer struct {
	endpoint string
	auth     TokenSource
	client   *http.Client
	// retryDelay is the pause before the single retry.
	retryDelay time.Duration
}

// NewRemoteRenderer validates baseURL (https, or http only for loopback) and
// builds a client. timeout bounds one whole request including cold start.
func NewRemoteRenderer(baseURL string, auth TokenSource, timeout time.Duration) (*RemoteRenderer, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return nil, errors.New("compile: invalid compiler URL")
	}
	host := u.Hostname()
	loopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return nil, errors.New("compile: compiler URL must be https")
	}
	if auth == nil {
		return nil, errors.New("compile: compiler auth is required")
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &RemoteRenderer{
		endpoint:   strings.TrimRight(u.String(), "/") + CompilePath,
		auth:       auth,
		client:     &http.Client{Timeout: timeout},
		retryDelay: 400 * time.Millisecond,
	}, nil
}

func (r *RemoteRenderer) Render(ctx context.Context, req RenderRequest) (*RenderResult, error) {
	body, contentType, err := encodeRenderRequest(req)
	if err != nil {
		return nil, err
	}
	res, err := r.attempt(ctx, body, contentType)
	if err != nil && ctx.Err() == nil && retryable(err) {
		select {
		case <-time.After(r.retryDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		res, err = r.attempt(ctx, body, contentType)
	}
	return res, err
}

// errUnavailable means the service could not be reached or answered with an
// infrastructure failure. Its text is generic on purpose.
type errUnavailable struct{ status int }

func (e errUnavailable) Error() string {
	if e.status != 0 {
		return fmt.Sprintf("compile: service unavailable (status %d)", e.status)
	}
	return "compile: service unavailable"
}

func retryable(err error) bool {
	var u errUnavailable
	if !errors.As(err, &u) {
		return false
	}
	return u.status == 0 || u.status == http.StatusTooManyRequests || u.status == http.StatusServiceUnavailable
}

func (r *RemoteRenderer) attempt(ctx context.Context, body []byte, contentType string) (*RenderResult, error) {
	token, err := r.auth.Token(ctx)
	if err != nil {
		return nil, errUnavailable{}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := r.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errUnavailable{}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		pdf, err := io.ReadAll(io.LimitReader(resp.Body, MaxPDFBytes+1))
		if err != nil || len(pdf) > MaxPDFBytes {
			return nil, errUnavailable{status: resp.StatusCode}
		}
		pages, convErr := strconv.Atoi(resp.Header.Get(HeaderPageCount))
		// The service is a separate trust domain: check what it sends back.
		if !bytes.HasPrefix(pdf, []byte("%PDF-")) || convErr != nil || pages < 1 || pages > MaxPages {
			return nil, errUnavailable{status: resp.StatusCode}
		}
		return &RenderResult{PDF: pdf, PageCount: pages}, nil
	case http.StatusUnprocessableEntity:
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&e)
		msg := strings.TrimSpace(e.Error)
		if msg == "" || len(msg) > 500 {
			msg = "compilation failed"
		}
		return nil, &CompileError{Message: msg}
	default:
		return nil, errUnavailable{status: resp.StatusCode}
	}
}

func encodeRenderRequest(req RenderRequest) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("compiler", req.Compiler); err != nil {
		return nil, "", err
	}
	if err := w.WriteField("tex", req.Tex); err != nil {
		return nil, "", err
	}
	for p, data := range req.Assets {
		part, err := w.CreateFormFile(AssetFieldPrefix+p, "asset")
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(data); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	if buf.Len() > MaxRequestBytes {
		return nil, "", invalid("request too large")
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}
