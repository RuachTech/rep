package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ruach-tech/rep/gateway/internal/config"
	repcrypto "github.com/ruach-tech/rep/gateway/internal/crypto"
	"github.com/ruach-tech/rep/gateway/internal/guardrails"
	"github.com/ruach-tech/rep/gateway/internal/health"
	"github.com/ruach-tech/rep/gateway/internal/inject"
	"github.com/ruach-tech/rep/gateway/pkg/payload"
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
