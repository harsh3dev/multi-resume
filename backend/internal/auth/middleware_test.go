package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// testJWKS serves a single EC public key in JWKS format.
type testJWKS struct {
	srv *httptest.Server
	key *ecdsa.PrivateKey
	kid string
	iss string
}

func newTestJWKS(t *testing.T) *testJWKS {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tj := &testJWKS{key: key, kid: "test-key-1", iss: "https://test.supabase.co/auth/v1"}
	tj.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pub := key.PublicKey
		jwks := map[string]any{"keys": []map[string]any{{
			"kty": "EC", "crv": "P-256", "kid": tj.kid, "use": "sig", "alg": "ES256",
			"x": base64.RawURLEncoding.EncodeToString(pub.X.FillBytes(make([]byte, 32))),
			"y": base64.RawURLEncoding.EncodeToString(pub.Y.FillBytes(make([]byte, 32))),
		}}}
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(tj.srv.Close)
	return tj
}

func (tj *testJWKS) token(t *testing.T, mutate func(*claims)) string {
	t.Helper()
	c := claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   uuid.NewString(),
		Issuer:    tj.iss,
		Audience:  jwt.ClaimStrings{"authenticated"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}, Email: "dev@example.com"}
	if mutate != nil {
		mutate(&c)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, c)
	tok.Header["kid"] = tj.kid
	signed, err := tok.SignedString(tj.key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func (tj *testJWKS) middleware(t *testing.T) *Middleware {
	t.Helper()
	m, err := NewMiddleware(context.Background(), tj.srv.URL, "https://test.supabase.co")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func okHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		id, ok := FromContext(r.Context())
		if !ok || id.UserID == uuid.Nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"email": id.Email})
	})
}

func TestValidTokenPasses(t *testing.T) {
	tj := newTestJWKS(t)
	m := tj.middleware(t)
	called := false
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+tj.token(t, nil))
	rec := httptest.NewRecorder()
	m.RequireAuth(okHandler(&called)).ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("valid token rejected: %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["email"] != "dev@example.com" {
		t.Fatalf("email claim missing: %v", body)
	}
}

func TestInvalidTokensRejected(t *testing.T) {
	tj := newTestJWKS(t)
	m := tj.middleware(t)
	cases := map[string]string{
		"missing header": "",
		"not bearer":     "Basic abc",
		"garbage":        "Bearer not-a-jwt",
		"expired": "Bearer " + tj.token(t, func(c *claims) {
			c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
		}),
		"wrong issuer": "Bearer " + tj.token(t, func(c *claims) {
			c.Issuer = "https://evil.example.com/auth/v1"
		}),
		"wrong audience": "Bearer " + tj.token(t, func(c *claims) {
			c.Audience = jwt.ClaimStrings{"service_role"}
		}),
		"no expiry": "Bearer " + tj.token(t, func(c *claims) {
			c.ExpiresAt = nil
		}),
		"bad subject": "Bearer " + tj.token(t, func(c *claims) {
			c.Subject = "not-a-uuid"
		}),
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			called := false
			req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			rec := httptest.NewRecorder()
			m.RequireAuth(okHandler(&called)).ServeHTTP(rec, req)
			if called || rec.Code != http.StatusUnauthorized {
				t.Fatalf("got %d called=%v, want 401", rec.Code, called)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != "unauthorized" {
				t.Fatalf("body %q", rec.Body.String())
			}
		})
	}
}

func TestWrongKeyRejected(t *testing.T) {
	tj := newTestJWKS(t)
	m := tj.middleware(t)
	// Sign with a different key than the JWKS serves.
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c := claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject: uuid.NewString(), Issuer: tj.iss,
		Audience:  jwt.ClaimStrings{"authenticated"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, c)
	tok.Header["kid"] = tj.kid
	signed, _ := tok.SignedString(other)
	called := false
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()
	m.RequireAuth(okHandler(&called)).ServeHTTP(rec, req)
	if called || rec.Code != http.StatusUnauthorized {
		t.Fatalf("forged key accepted: %d", rec.Code)
	}
}

func TestHS256AlgConfusionRejected(t *testing.T) {
	tj := newTestJWKS(t)
	m := tj.middleware(t)
	// HS256-signed token must be rejected even with a known secret.
	c := claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject: uuid.NewString(), Issuer: tj.iss,
		Audience:  jwt.ClaimStrings{"authenticated"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	signed, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte("secret"))
	called := false
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()
	m.RequireAuth(okHandler(&called)).ServeHTTP(rec, req)
	if called || rec.Code != http.StatusUnauthorized {
		t.Fatalf("HS256 token accepted: %d", rec.Code)
	}
}

var _ = big.NewInt
