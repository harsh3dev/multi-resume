// Package auth verifies Supabase-issued JWTs against the project's JWKS.
// Go never calls Supabase Auth; it only verifies tokens statelessly.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Identity is the verified caller. UserID comes from the JWT sub claim.
type Identity struct {
	UserID uuid.UUID
	Email  string
}

type ctxKey struct{}

// FromContext returns the verified identity, or false when absent.
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// WithIdentity injects an identity into a context. Intended for tests of
// handlers downstream of RequireAuth — production code must go through the
// middleware.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// Middleware verifies Bearer tokens against Supabase's JWKS endpoint. Keys are
// fetched once and auto-refreshed by keyfunc (cached in memory).
type Middleware struct {
	keyfunc jwt.Keyfunc
	issuer  string
}

// NewMiddleware builds a Middleware for the given JWKS URL and Supabase URL
// (e.g. https://<project>.supabase.co); the issuer is <url>/auth/v1.
func NewMiddleware(ctx context.Context, jwksURL, supabaseURL string) (*Middleware, error) {
	if jwksURL == "" || supabaseURL == "" {
		return nil, errors.New("jwks URL and supabase URL are required")
	}
	kf, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	return &Middleware{keyfunc: kf.Keyfunc, issuer: strings.TrimSuffix(supabaseURL, "/") + "/auth/v1"}, nil
}

// RequireAuth rejects requests without a valid Supabase access token.
// Anything other than a fully valid token gets the same generic 401.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := m.verify(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
	})
}

type claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

func (m *Middleware) verify(r *http.Request) (Identity, error) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return Identity{}, errors.New("missing bearer token")
	}
	var c claims
	_, err := jwt.ParseWithClaims(strings.TrimPrefix(header, "Bearer "), &c, m.keyfunc,
		jwt.WithValidMethods([]string{"ES256", "RS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience("authenticated"),
	)
	if err != nil {
		return Identity{}, err
	}
	uid, err := uuid.Parse(c.Subject)
	if err != nil {
		return Identity{}, errors.New("invalid subject")
	}
	return Identity{UserID: uid, Email: c.Email}, nil
}
