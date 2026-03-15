package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ruachtech/rep/gateway/internal/config"
	repcrypto "github.com/ruachtech/rep/gateway/internal/crypto"
	"github.com/ruachtech/rep/gateway/internal/guardrails"
	"github.com/ruachtech/rep/gateway/internal/health"
	"github.com/ruachtech/rep/gateway/internal/inject"
	"github.com/ruachtech/rep/gateway/pkg/payload"
)

// buildTestMux creates a test HTTP mux from the given classified vars.
func buildTestMux(t *testing.T, vars *config.ClassifiedVars, staticDir string, hotReload bool) *http.ServeMux {
	t.Helper()

	logger := slog.Default()
	gr := guardrails.Scan(vars, logger)

	keys, err := repcrypto.GenerateKeys()
	if err != nil {
		t.Fatalf("key gen error: %v", err)
	}

	builder := payload.NewBuilder(keys, "0.1.0-test", hotReload)
	p, err := builder.Build(vars)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	scriptTag, err := p.ScriptTag()
	if err != nil {
		t.Fatalf("script tag error: %v", err)
	}

	fs := http.FileServer(http.Dir(staticDir))
	injector := inject.New(fs, scriptTag, logger)

	mux := http.NewServeMux()
	healthHandler := health.NewHandler("0.1.0-test", vars, gr, time.Now())
	mux.Handle("/rep/health", healthHandler)

	if len(vars.Sensitive) > 0 {
		skHandler := repcrypto.NewSessionKeyHandler(
			keys.EncryptionKey,
			30*time.Second,
			10,
			nil,
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

	mux.Handle("/", injector)

	return mux
}

func TestServer_HealthEndpoint(t *testing.T) {
	vars := &config.ClassifiedVars{
		Public: []config.Variable{
			{Name: "TEST", Value: "value", Tier: config.TierPublic, OriginalKey: "REP_PUBLIC_TEST"},
		},
	}

	mux := buildTestMux(t, vars, "../../testdata/static", false)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/rep/health")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var healthResp health.Response
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if healthResp.Status != "healthy" {
		t.Errorf("expected healthy, got %s", healthResp.Status)
	}
	if healthResp.Variables.Public != 1 {
		t.Errorf("expected 1 public var, got %d", healthResp.Variables.Public)
	}
}

func TestServer_HTMLInjection(t *testing.T) {
	vars := &config.ClassifiedVars{
		Public: []config.Variable{
			{Name: "APP_NAME", Value: "TestApp", Tier: config.TierPublic, OriginalKey: "REP_PUBLIC_APP_NAME"},
		},
	}

	mux := buildTestMux(t, vars, "../../testdata/static", false)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	body := string(bodyBytes)

	if !containsStr(body, `id="__rep__"`) {
		t.Error("expected injected script tag in HTML response")
	}
	if !containsStr(body, "APP_NAME") {
		t.Error("expected APP_NAME in injected payload")
	}
	if !containsStr(body, "TestApp") {
		t.Error("expected TestApp value in injected payload")
	}
}

func TestServer_SessionKeyEndpoint(t *testing.T) {
	vars := &config.ClassifiedVars{
		Sensitive: []config.Variable{
			{Name: "SECRET", Value: "my-secret", Tier: config.TierSensitive, OriginalKey: "REP_SENSITIVE_SECRET"},
		},
	}

	mux := buildTestMux(t, vars, "../../testdata/static", false)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/rep/session-key")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var skResp repcrypto.SessionKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&skResp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if skResp.Key == "" {
		t.Error("expected non-empty session key")
	}
}

func TestServer_NoSessionKeyWithoutSensitive(t *testing.T) {
	vars := &config.ClassifiedVars{
		Public: []config.Variable{
			{Name: "ONLY", Value: "value", Tier: config.TierPublic, OriginalKey: "REP_PUBLIC_ONLY"},
		},
	}

	mux := buildTestMux(t, vars, "../../testdata/static", false)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/rep/session-key")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// createFileServer / SPA-fallback tests
// ---------------------------------------------------------------------------

// newFileServerHandler constructs the SPA-aware file server handler from
// createFileServer using only the staticDir — no crypto, no injection, no
// payload. This isolates routing logic from the rest of the gateway stack.
func newFileServerHandler(t *testing.T, staticDir string) http.Handler {
	t.Helper()
	s := &Server{
		cfg:    &config.Config{StaticDir: staticDir},
		logger: slog.Default(),
	}
	return s.createFileServer()
}

func TestFileServer_RootServesIndexHTML(t *testing.T) {
	handler := newFileServerHandler(t, "../../testdata/static")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: expected 200, got %d", rec.Code)
	}
	if !containsStr(rec.Body.String(), "REP Gateway Test") {
		t.Error("GET /: expected root index.html content")
	}
}

func TestFileServer_StaticAssetServedDirectly(t *testing.T) {
	// Requests with a file extension should be served as-is from disk.
	handler := newFileServerHandler(t, "../../testdata/static")

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /style.css: expected 200, got %d", rec.Code)
	}
	if !containsStr(rec.Body.String(), "color") {
		t.Error("GET /style.css: expected CSS content")
	}
}

func TestFileServer_PrerenderedDirWithTrailingSlash(t *testing.T) {
	// A directory containing index.html (e.g. Next.js trailingSlash, Gatsby)
	// should serve that page directly, not the root index.html.
	handler := newFileServerHandler(t, "../../testdata/static")

	req := httptest.NewRequest(http.MethodGet, "/signin/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /signin/: expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !containsStr(body, "Sign In") {
		t.Error("GET /signin/: expected signin/index.html content, got root index.html")
	}
}

func TestFileServer_PrerenderedDirWithoutTrailingSlash(t *testing.T) {
	// /signin (no slash) where signin/index.html exists on disk.
	// http.FileServer issues a 301 → /signin/ which then serves the page.
	handler := newFileServerHandler(t, "../../testdata/static")

	req := httptest.NewRequest(http.MethodGet, "/signin", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// http.FileServer redirects directories without trailing slash to add one.
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("GET /signin: expected 301 redirect, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	// httptest returns relative redirects; http.FileServer may use "signin/"
	// or "/signin/" depending on context.
	if loc != "/signin/" && loc != "signin/" {
		t.Errorf("GET /signin: expected redirect to /signin/ or signin/, got %s", loc)
	}
}

func TestFileServer_SPAFallbackForUnknownRoute(t *testing.T) {
	// An extension-less path with no matching directory on disk should
	// fall back to the root index.html (standard SPA behaviour).
	handler := newFileServerHandler(t, "../../testdata/static")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/settings", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /dashboard/settings: expected 200, got %d", rec.Code)
	}
	if !containsStr(rec.Body.String(), "REP Gateway Test") {
		t.Error("GET /dashboard/settings: expected root index.html via SPA fallback")
	}
}

func TestFileServer_SPAFallbackForTrailingSlashNoDir(t *testing.T) {
	// An extension-less path with trailing slash but no matching directory
	// should also fall back to root index.html (pure SPA — Vite, CRA).
	handler := newFileServerHandler(t, "../../testdata/static")

	req := httptest.NewRequest(http.MethodGet, "/nonexistent/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /nonexistent/: expected 200, got %d", rec.Code)
	}
	if !containsStr(rec.Body.String(), "REP Gateway Test") {
		t.Error("GET /nonexistent/: expected root index.html via SPA fallback")
	}
}

func TestFileServer_MissingStaticAsset404(t *testing.T) {
	// A request for a non-existent file WITH an extension should 404,
	// not fall back to index.html.
	handler := newFileServerHandler(t, "../../testdata/static")

	req := httptest.NewRequest(http.MethodGet, "/missing.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /missing.js: expected 404, got %d", rec.Code)
	}
}

func TestFileServer_DirWithoutIndexHTMLFallsBack(t *testing.T) {
	// A directory that exists but does NOT contain index.html should
	// fall back to root index.html (SPA behaviour), not list the dir.
	t.TempDir() // just for cleanup
	dir := t.TempDir()

	// Create root index.html.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"),
		[]byte("<html><body>Root</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a subdirectory without index.html.
	if err := os.MkdirAll(filepath.Join(dir, "noindex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "noindex", "data.txt"),
		[]byte("not html"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := newFileServerHandler(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/noindex/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /noindex/: expected 200, got %d", rec.Code)
	}
	if !containsStr(rec.Body.String(), "Root") {
		t.Error("GET /noindex/: expected root index.html via SPA fallback, not dir listing")
	}
}

// containsStr is a helper to avoid importing strings just for Contains.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
