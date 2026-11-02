// Command compiler is the standalone LaTeX compile service. It runs in its own
// container with the TeX toolchain, holds no secrets, and is called by the main
// API server (see internal/compile.RemoteRenderer).
//
// Environment:
//
//	PORT                   listen port (default 8080)
//	COMPILER_AUTH          "iam" when the platform enforces authentication
//	                       (Cloud Run with --no-allow-unauthenticated); the
//	                       service then performs no auth of its own
//	COMPILER_TOKEN         shared-secret bearer, required unless COMPILER_AUTH=iam
//	MAX_PARALLEL_COMPILES  concurrent jobs per instance (default 1)
//	RECYCLE_AFTER_JOBS     replace the instance after N jobs (default 50, 0=off)
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/harsh3dev/multi-resume/backend/internal/compile"
	"github.com/harsh3dev/multi-resume/backend/internal/compilesvc"
)

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Fail closed: an unauthenticated compile endpoint must be a deliberate choice.
	token := os.Getenv("COMPILER_TOKEN")
	if token == "" && os.Getenv("COMPILER_AUTH") != "iam" {
		logger.Error("set COMPILER_TOKEN, or COMPILER_AUTH=iam when the platform authenticates callers")
		os.Exit(1)
	}
	// Fail fast if the image is missing its toolchain.
	for _, bin := range []string{"latexmk", "prlimit", "pdfinfo"} {
		if _, err := exec.LookPath(bin); err != nil {
			logger.Error("required binary missing", "binary", bin)
			os.Exit(1)
		}
	}

	renderer, err := compile.NewLocalRenderer(compile.LocalOptions{
		Runner:            compile.DirectRunner{},
		MaxParallel:       envInt("MAX_PARALLEL_COMPILES", 1),
		DisabledCompilers: []string{compile.CompilerLuaLatex},
	})
	if err != nil {
		logger.Error("renderer setup failed", "error", err)
		os.Exit(1)
	}

	stop := make(chan struct{})
	var once sync.Once
	recycle := func() { once.Do(func() { close(stop) }) }

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr: ":" + port,
		Handler: compilesvc.New(compilesvc.Config{
			Renderer:         renderer,
			Token:            token,
			Recycle:          recycle,
			RecycleAfterJobs: int64(envInt("RECYCLE_AFTER_JOBS", 50)),
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	go func() {
		logger.Info("compiler starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("compiler stopped", "error", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
	case <-stop:
		logger.Info("recycling instance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
