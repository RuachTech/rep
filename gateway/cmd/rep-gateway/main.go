// Package main provides the REP Gateway binary.
//
// The REP Gateway reads REP_* environment variables, classifies them into
// PUBLIC, SENSITIVE, and SERVER tiers, and injects a signed JSON payload
// into HTML responses served by an upstream static file server.
//
// Usage:
//
//	rep-gateway [flags]
//
// Modes:
//
//	proxy     (default) Reverse proxy to an upstream server, injecting into HTML responses.
//	embedded  Serve static files directly with injection — no upstream server needed.
//
// See https://github.com/ruach-tech/rep for the full specification.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ruach-tech/rep/gateway/internal/config"
	"github.com/ruach-tech/rep/gateway/internal/server"
)

// version is set at build time via -ldflags.
var version = "0.1.0-dev"

func main() {
	cfg, err := config.Parse(os.Args[1:], version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rep-gateway: %v\n", err)
		os.Exit(1)
	}

	if cfg.ShowVersion {
		fmt.Printf("rep-gateway %s\n", version)
		os.Exit(0)
	}

	// Configure structured logger.
	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel()})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel()})
	}
	logger := slog.New(handler)

	// Create and start the server.
	srv, err := server.New(cfg, logger, version)
	if err != nil {
		logger.Error("failed to initialise gateway", "error", err)
		os.Exit(1)
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// SIGHUP triggers config reload (hot reload signal mode).
	if cfg.HotReload && cfg.HotReloadMode == "signal" {
		sighup := make(chan os.Signal, 1)
		signal.Notify(sighup, syscall.SIGHUP)
		go func() {
			for range sighup {
				logger.Info("received SIGHUP, reloading configuration")
				if err := srv.Reload(); err != nil {
					logger.Error("config reload failed", "error", err)
				}
			}
		}()
	}

	if err := srv.Start(ctx); err != nil {
		logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
