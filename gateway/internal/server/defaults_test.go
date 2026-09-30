package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ruachtech/rep/gateway/internal/config"
	"github.com/ruachtech/rep/gateway/internal/manifest"
	"github.com/ruachtech/rep/gateway/pkg/payload"
)

// startupLog returns the attributes of the rep.gateway.started log line.
func startupLog(t *testing.T, logs *bytes.Buffer) map[string]any {
	t.Helper()
	sc := bufio.NewScanner(logs)
	for sc.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(sc.Bytes(), &entry); err != nil {
			t.Fatalf("decoding log line %q: %v", sc.Text(), err)
		}
		if entry["msg"] == "rep.gateway.started" {
			return entry
		}
	}
	t.Fatal("no rep.gateway.started log line")
	return nil
}

// injectedPublic serves "/" through the gateway and returns the public map of
// the payload it injected.
func injectedPublic(t *testing.T, s *Server) map[string]string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	_, tag, ok := bytes.Cut(rec.Body.Bytes(), []byte(`<script id="__rep__"`))
	_, tag, ok2 := bytes.Cut(tag, []byte(">"))
	body, _, ok3 := bytes.Cut(tag, []byte("</script>"))
	if !ok || !ok2 || !ok3 {
		t.Fatalf("no REP payload in response:\n%s", rec.Body.String())
	}

	var p payload.Payload
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("decoding payload %q: %v", body, err)
	}
	return p.Public
}

// newTestServer builds a gateway from the given manifest declarations.
func newTestServer(decls map[string]*manifest.VarDecl, logger *slog.Logger) (*Server, error) {
	return New(&config.Config{
		Mode:      "embedded",
		StaticDir: "../../testdata/static",
		Manifest:  &manifest.Manifest{Variables: decls},
	}, logger, "0.0.0-test")
}

func TestNew_ManifestDefaults(t *testing.T) {
	ptr := func(s string) *string { return &s }

	// Every case starts from the manifest shape StageFlow ships: two required
	// variables that are set, and a csv flag list that is optional.
	tests := []struct {
		name          string
		flagsEnv      *string // REP_PUBLIC_FEATURE_FLAGS; nil = unset
		flagsDefault  string
		extra         map[string]*manifest.VarDecl
		wantErr       string
		wantFlags     string
		wantDefaulted float64
	}{
		{name: "empty default is injected and counted", flagsDefault: "", wantFlags: "", wantDefaulted: 1},
		{name: "non-empty default is injected", flagsDefault: "dark-mode,beta", wantFlags: "dark-mode,beta", wantDefaulted: 1},
		{name: "environment value wins over the default", flagsEnv: ptr("new-checkout"), flagsDefault: "dark-mode", wantFlags: "new-checkout"},
		{name: "environment empty string wins over a non-empty default", flagsEnv: ptr(""), flagsDefault: "dark-mode", wantFlags: ""},
		{
			name:    "required and unset is still an error despite a default",
			extra:   map[string]*manifest.VarDecl{"REGION": {Tier: "public", Required: true, Default: "eu", HasDefault: true}},
			wantErr: `required variable "REGION" is not set`,
		},
		{
			name:    "a default that fails its declared type is refused",
			extra:   map[string]*manifest.VarDecl{"STATUS_URL": {Tier: "public", Type: "url", HasDefault: true}},
			wantErr: `variable "STATUS_URL" must be a valid URL`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearREPEnv(t)
			t.Setenv("REP_PUBLIC_API_URL", "https://api.example.com")
			t.Setenv("REP_PUBLIC_ENV_NAME", "production")
			if tt.flagsEnv != nil {
				t.Setenv("REP_PUBLIC_FEATURE_FLAGS", *tt.flagsEnv)
			}
			decls := map[string]*manifest.VarDecl{
				"API_URL":       {Tier: "public", Type: "url", Required: true},
				"ENV_NAME":      {Tier: "public", Type: "string", Required: true},
				"FEATURE_FLAGS": {Tier: "public", Type: "csv", Default: tt.flagsDefault, HasDefault: true},
			}
			maps.Copy(decls, tt.extra)

			var logs bytes.Buffer
			s, err := newTestServer(decls, slog.New(slog.NewJSONHandler(&logs, nil)))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("New() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}

			want := map[string]string{
				"API_URL":       "https://api.example.com",
				"ENV_NAME":      "production",
				"FEATURE_FLAGS": tt.wantFlags,
			}
			if got := injectedPublic(t, s); !maps.Equal(got, want) {
				t.Errorf("payload public = %q, want %q", got, want)
			}

			entry := startupLog(t, &logs)
			if entry["public_vars"] != float64(len(want)) {
				t.Errorf("startup log public_vars = %v, want %d", entry["public_vars"], len(want))
			}
			if entry["defaulted_vars"] != tt.wantDefaulted {
				t.Errorf("startup log defaulted_vars = %v, want %v", entry["defaulted_vars"], tt.wantDefaulted)
			}
		})
	}
}

// A default must survive a reload, and must not look like a change to the
// poller on every tick.
func TestReload_KeepsManifestDefaults(t *testing.T) {
	clearREPEnv(t)
	t.Setenv("REP_PUBLIC_API_URL", "https://api.example.com")

	s, err := newTestServer(map[string]*manifest.VarDecl{
		"FEATURE_FLAGS": {Tier: "public", Type: "csv", Default: "", HasDefault: true},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	polled, err := s.readVars()
	if err != nil {
		t.Fatalf("readVars() unexpected error: %v", err)
	}
	if varsChanged(s.vars, polled) {
		t.Error("poller sees a change on an unchanged environment")
	}

	if err := s.Reload(); err != nil {
		t.Fatalf("Reload() unexpected error: %v", err)
	}
	if _, ok := injectedPublic(t, s)["FEATURE_FLAGS"]; !ok {
		t.Error("FEATURE_FLAGS missing from the payload after reload")
	}
}
