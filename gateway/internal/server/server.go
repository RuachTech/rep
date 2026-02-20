// Package server orchestrates the REP gateway lifecycle.
//
// It implements the startup sequence defined in REP-RFC-0001 §4.2:
//  1. Read and classify environment variables
//  2. Run guardrails
//  3. Generate ephemeral crypto keys
//  4. Build the payload
//  5. Register HTTP handlers
//  6. Start accepting connections
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/ruach-tech/rep/gateway/internal/config"
	repcrypto "github.com/ruach-tech/rep/gateway/internal/crypto"
	"github.com/ruach-tech/rep/gateway/internal/guardrails"
	"github.com/ruach-tech/rep/gateway/internal/health"
	"github.com/ruach-tech/rep/gateway/internal/hotreload"
	"github.com/ruach-tech/rep/gateway/internal/inject"
	"github.com/ruach-tech/rep/gateway/pkg/payload"
)

// Server is the REP gateway server.
type Server struct {
	cfg     *config.Config
	logger  *slog.Logger
	version string

	vars          *config.ClassifiedVars
	keys          *repcrypto.Keys
	injector      *inject.Middleware
	hotReloadHub  *hotreload.Hub
	httpServer    *http.Server
	healthServer  *http.Server // Optional separate health server.
	startTime     time.Time
}

// New creates and initialises a new REP gateway server.
// This performs steps 1–9 of the startup sequence (§4.2).
func New(cfg *config.Config, logger *slog.Logger, version string) (*Server, error) {
	s := &Server{
		cfg:       cfg,
		logger:    logger,
		version:   version,
		startTime: time.Now(),
	}

	// Step 1–2: Read and classify environment variables.
	logger.Info("reading environment variables")
	vars, err := config.ReadAndClassify()
	if err != nil {
		return nil, fmt.Errorf("classifying variables: %w", err)
	}
	s.vars = vars

	// Step 3–4: Run secret detection guardrails.
	logger.Info("running guardrail scan on PUBLIC tier variables")
	gr := guardrails.Scan(vars, logger)

	if gr.HasWarnings() && cfg.Strict {
		return nil, fmt.Errorf(
			"guardrail scan found %d warning(s) and --strict is enabled; refusing to start",
			len(gr.Warnings),
		)
	}

	// Step 5: Generate ephemeral crypto keys.
	logger.Info("generating ephemeral cryptographic keys")
	keys, err := repcrypto.GenerateKeys()
	if err != nil {
		return nil, fmt.Errorf("generating keys: %w", err)
	}
	s.keys = keys

	// Step 6–7: Build the payload and render the script tag.
	builder := payload.NewBuilder(keys, version, cfg.HotReload)
	p, err := builder.Build(vars)
	if err != nil {
		return nil, fmt.Errorf("building payload: %w", err)
	}

	scriptTag, err := p.ScriptTag()
	if err != nil {
		return nil, fmt.Errorf("rendering script tag: %w", err)
	}

	// Step 8: Create the upstream handler (proxy or file server).
	var upstream http.Handler
	switch cfg.Mode {
	case "proxy":
		upstream, err = s.createReverseProxy()
		if err != nil {
			return nil, fmt.Errorf("creating reverse proxy: %w", err)
		}
	case "embedded":
		upstream = s.createFileServer()
	}

	// Create the injection middleware wrapping the upstream.
	s.injector = inject.New(upstream, scriptTag, logger)

	// Step 9: Create hot reload hub if enabled.
	if cfg.HotReload {
		s.hotReloadHub = hotreload.NewHub(logger)
	}

	// Build the HTTP mux.
	mux := http.NewServeMux()

	// Health check (§4.5).
	healthHandler := health.NewHandler(version, vars, gr, s.startTime)
	mux.Handle("/rep/health", healthHandler)

	// Session key endpoint (§4.4) — only if sensitive vars exist.
	if len(vars.Sensitive) > 0 {
		skHandler := repcrypto.NewSessionKeyHandler(
			keys.EncryptionKey,
			cfg.SessionKeyTTL,
			cfg.SessionKeyMaxRate,
			cfg.AllowedOrigins,
			logger,
		)
		mux.HandleFunc("/rep/session-key", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				skHandler.CORSPreflight(w, r)
				return
			}
			skHandler.ServeHTTP(w, r)
		})
	}

	// Hot reload SSE endpoint (§4.6).
	if cfg.HotReload && s.hotReloadHub != nil {
		mux.Handle("/rep/changes", hotreload.NewHandler(s.hotReloadHub))
	}

	// All other requests go through the injection middleware.
	mux.Handle("/", s.injector)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Optional separate health server.
	if cfg.HealthPort > 0 && cfg.HealthPort != cfg.Port {
		healthMux := http.NewServeMux()
		healthMux.Handle("/rep/health", healthHandler)
		s.healthServer = &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.HealthPort),
			Handler: healthMux,
		}
	}

	// Step 10: Log startup summary.
	logger.Info("rep.gateway.started",
		"version", version,
		"mode", cfg.Mode,
		"port", cfg.Port,
		"public_vars", len(vars.Public),
		"sensitive_vars", len(vars.Sensitive),
		"server_vars", len(vars.Server),
		"guardrail_warnings", len(gr.Warnings),
		"hot_reload", cfg.HotReload,
		"strict", cfg.Strict,
	)

	return s, nil
}

// Start begins serving HTTP requests. Blocks until context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	// Start optional separate health server.
	if s.healthServer != nil {
		go func() {
			s.logger.Info("health server starting", "addr", s.healthServer.Addr)
			if err := s.healthServer.ListenAndServe(); err != http.ErrServerClosed {
				s.logger.Error("health server error", "error", err)
			}
		}()
	}

	// Start main server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("gateway listening", "addr", s.httpServer.Addr)

		var err error
		if s.cfg.TLSCert != "" && s.cfg.TLSKey != "" {
			err = s.httpServer.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
		} else {
			err = s.httpServer.ListenAndServe()
		}

		if err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal or error.
	select {
	case <-ctx.Done():
		s.logger.Info("shutting down gateway")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if s.healthServer != nil {
			s.healthServer.Shutdown(shutdownCtx)
		}
		return s.httpServer.Shutdown(shutdownCtx)

	case err := <-errCh:
		return err
	}
}

// Reload re-reads environment variables and rebuilds the payload.
// Used for hot reload (SIGHUP signal mode).
func (s *Server) Reload() error {
	s.logger.Info("reloading configuration")

	// Re-read and classify.
	vars, err := config.ReadAndClassify()
	if err != nil {
		return fmt.Errorf("re-classifying variables: %w", err)
	}

	// Detect changes and broadcast.
	if s.hotReloadHub != nil {
		s.broadcastChanges(s.vars, vars)
	}

	// Rebuild payload.
	builder := payload.NewBuilder(s.keys, s.version, s.cfg.HotReload)
	p, err := builder.Build(vars)
	if err != nil {
		return fmt.Errorf("rebuilding payload: %w", err)
	}

	scriptTag, err := p.ScriptTag()
	if err != nil {
		return fmt.Errorf("re-rendering script tag: %w", err)
	}

	// Update the injector.
	s.injector.UpdateScriptTag(scriptTag)
	s.vars = vars

	s.logger.Info("configuration reloaded",
		"public_vars", len(vars.Public),
		"sensitive_vars", len(vars.Sensitive),
		"server_vars", len(vars.Server),
	)

	return nil
}

// broadcastChanges compares old and new variables and emits SSE events.
func (s *Server) broadcastChanges(oldVars, newVars *config.ClassifiedVars) {
	oldPublic := oldVars.PublicMap()
	newPublic := newVars.PublicMap()

	// Detect updates and additions.
	for key, newVal := range newPublic {
		if oldVal, exists := oldPublic[key]; !exists || oldVal != newVal {
			s.hotReloadHub.Broadcast(hotreload.Event{
				Type:  "rep:config:update",
				Key:   key,
				Tier:  "public",
				Value: newVal,
			})
		}
	}

	// Detect deletions.
	for key := range oldPublic {
		if _, exists := newPublic[key]; !exists {
			s.hotReloadHub.Broadcast(hotreload.Event{
				Type: "rep:config:delete",
				Key:  key,
				Tier: "public",
			})
		}
	}
}

// createReverseProxy sets up a reverse proxy to the upstream server.
func (s *Server) createReverseProxy() (http.Handler, error) {
	upstream := s.cfg.Upstream
	if !strings.HasPrefix(upstream, "http") {
		upstream = "http://" + upstream
	}

	target, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("parsing upstream URL %q: %w", upstream, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	return proxy, nil
}

// createFileServer sets up a static file server for embedded mode.
func (s *Server) createFileServer() http.Handler {
	dir := s.cfg.StaticDir
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}

	s.logger.Info("serving static files", "directory", absDir)

	fs := http.FileServer(http.Dir(absDir))

	// Wrap with SPA fallback: if a file is not found, serve index.html.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file directly.
		// For SPA routing, we want to serve index.html for non-file paths.
		path := r.URL.Path
		if path == "/" || filepath.Ext(path) != "" {
			fs.ServeHTTP(w, r)
			return
		}

		// For paths without extensions (likely SPA routes), serve index.html.
		r.URL.Path = "/"
		fs.ServeHTTP(w, r)
	})
}
