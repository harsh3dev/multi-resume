package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/harsh3dev/multi-resume/backend/internal/api"
	"github.com/harsh3dev/multi-resume/backend/internal/auth"
	"github.com/harsh3dev/multi-resume/backend/internal/compile"
	"github.com/harsh3dev/multi-resume/backend/internal/config"
	"github.com/harsh3dev/multi-resume/backend/internal/storage"
	"github.com/harsh3dev/multi-resume/backend/internal/store"
	"github.com/harsh3dev/multi-resume/backend/internal/tailor"
)

// r2Store adapts the R2 client to the compiler's object-store contract.
// Compiled PDFs are stored with the PDF content type.
type r2Store struct{ c *storage.Client }

func (s r2Store) Get(ctx context.Context, key string) ([]byte, error) {
	return s.c.GetObject(ctx, key)
}

func (s r2Store) Put(ctx context.Context, key string, data []byte) error {
	return s.c.PutObject(ctx, key, "application/pdf", data)
}

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Sentry is opt-in via env; events are scrubbed of PII before sending.
	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn: dsn,
			BeforeSend: func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
				event.User = sentry.User{}           // no user PII
				event.Request = nil                  // no headers (Authorization)
				return event
			},
		}); err != nil {
			logger.Warn("sentry init failed")
		}
		defer sentry.Flush(2 * time.Second)
	}

	var deps api.Deps
	if cfg.SupabaseURL == "" || cfg.SupabaseJWKSURL == "" {
		logger.Warn("SUPABASE_URL/SUPABASE_JWKS_URL unset — API routes will return 503")
	} else {
		mw, err := auth.NewMiddleware(context.Background(), cfg.SupabaseJWKSURL, cfg.SupabaseURL)
		if err != nil {
			logger.Error("auth setup failed", "error", err)
			os.Exit(1)
		}
		deps.Auth = mw
	}
	if cfg.DatabaseURL == "" {
		logger.Warn("DATABASE_URL unset — DB-backed routes will return 503")
	} else {
		pool, err := store.NewPool(context.Background(), cfg.DatabaseURL)
		if err != nil {
			logger.Error("database connection failed", "error", err)
			os.Exit(1)
		}
		defer pool.Close()
		deps.Store = store.New(pool)
		deps.Ping = pool.Ping
	}
	if cfg.LLMAPIKey == "" {
		logger.Warn("LLM_API_KEY unset — tailor route will return 503")
	} else {
		deps.Tailor = tailor.NewOpenAIStreamer(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)
	}
	// R2 storage + compile pipeline. Compiles run latexmk inside the
	// bubblewrap sandbox and store PDFs in R2; without R2 config the
	// compile-dependent routes degrade to 503.
	if cfg.R2AccountID == "" || cfg.R2AccessKeyID == "" || cfg.R2SecretAccessKey == "" || cfg.R2Bucket == "" {
		logger.Warn("R2 config unset — compile, asset, and PDF routes will return 503")
	} else {
		r2, err := storage.New(storage.Config{
			AccountID:       cfg.R2AccountID,
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
			Bucket:          cfg.R2Bucket,
		})
		if err != nil {
			logger.Error("r2 setup failed", "error", err)
			os.Exit(1)
		}
		deps.Storage = r2
		deps.Presign = r2
		opts := compile.Options{
			// Shared across instances and restarts (sidecar objects in R2).
			Cache: compile.NewObjectCache(r2Store{r2}),
			Store: r2Store{r2},
		}
		if cfg.CompilerURL != "" {
			// Production: the TeX toolchain lives in the separate compiler service.
			var tokens compile.TokenSource
			switch cfg.CompilerAuth {
			case "iam":
				tokens = &compile.MetadataIDToken{Audience: cfg.CompilerURL}
			case "bearer":
				if cfg.CompilerToken == "" {
					logger.Error("COMPILER_AUTH=bearer needs COMPILER_TOKEN")
					os.Exit(1)
				}
				tokens = compile.StaticToken(cfg.CompilerToken)
			default:
				logger.Error("COMPILER_URL is set: set COMPILER_AUTH to iam or bearer")
				os.Exit(1)
			}
			remote, err := compile.NewRemoteRenderer(cfg.CompilerURL, tokens, 60*time.Second)
			if err != nil {
				logger.Error("compiler client setup failed", "error", err)
				os.Exit(1)
			}
			opts.Renderer = remote
		} else {
			// Local dev: compile in-process under bubblewrap.
			opts.Runner = compile.BwrapRunner{}
			opts.MaxParallel = cfg.MaxParallelCompiles
		}
		comp, err := compile.New(opts)
		if err != nil {
			logger.Error("compile setup failed", "error", err)
			os.Exit(1)
		}
		deps.Compiler = comp
	}
	if missing := cfg.Missing(); len(missing) > 0 {
		logger.Warn("configuration incomplete", "missing", missing)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.Router(deps),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second, // tailor streams
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	logger.Info("server stopped cleanly")
}
