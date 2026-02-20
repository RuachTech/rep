package config

import (
	"os"
	"testing"
)

// clearREPEnv removes all REP_* env vars to provide a clean test environment.
func clearREPEnv(t *testing.T) {
	t.Helper()
	for _, env := range os.Environ() {
		for i := 0; i < len(env); i++ {
			if env[i] == '=' {
				key := env[:i]
				if len(key) >= 4 && key[:4] == "REP_" {
					t.Setenv(key, "")
					if err := os.Unsetenv(key); err != nil {
						t.Errorf("failed to unsetenv %q: %v", key, err)
					}
				}
				break
			}
		}
	}
}

func TestReadAndClassify_PublicVars(t *testing.T) {
	clearREPEnv(t)
	t.Setenv("REP_PUBLIC_API_URL", "https://api.example.com")

	vars, err := ReadAndClassify()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vars.Public) != 1 {
		t.Fatalf("expected 1 public var, got %d", len(vars.Public))
	}
	if vars.Public[0].Name != "API_URL" {
		t.Errorf("expected Name=API_URL, got %s", vars.Public[0].Name)
	}
	if vars.Public[0].Value != "https://api.example.com" {
		t.Errorf("expected Value=https://api.example.com, got %s", vars.Public[0].Value)
	}
	if vars.Public[0].Tier != TierPublic {
		t.Errorf("expected Tier=TierPublic, got %v", vars.Public[0].Tier)
	}
	if vars.Public[0].OriginalKey != "REP_PUBLIC_API_URL" {
		t.Errorf("expected OriginalKey=REP_PUBLIC_API_URL, got %s", vars.Public[0].OriginalKey)
	}
}

func TestReadAndClassify_SensitiveVars(t *testing.T) {
	clearREPEnv(t)
	t.Setenv("REP_SENSITIVE_ANALYTICS_KEY", "UA-12345")

	vars, err := ReadAndClassify()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vars.Sensitive) != 1 {
		t.Fatalf("expected 1 sensitive var, got %d", len(vars.Sensitive))
	}
	if vars.Sensitive[0].Name != "ANALYTICS_KEY" {
		t.Errorf("expected Name=ANALYTICS_KEY, got %s", vars.Sensitive[0].Name)
	}
	if vars.Sensitive[0].Tier != TierSensitive {
		t.Errorf("expected TierSensitive, got %v", vars.Sensitive[0].Tier)
	}
}

func TestReadAndClassify_ServerVars(t *testing.T) {
	clearREPEnv(t)
	t.Setenv("REP_SERVER_DB_PASSWORD", "s3cret")

	vars, err := ReadAndClassify()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vars.Server) != 1 {
		t.Fatalf("expected 1 server var, got %d", len(vars.Server))
	}
	if vars.Server[0].Name != "DB_PASSWORD" {
		t.Errorf("expected Name=DB_PASSWORD, got %s", vars.Server[0].Name)
	}
	if vars.Server[0].Tier != TierServer {
		t.Errorf("expected TierServer, got %v", vars.Server[0].Tier)
	}
}

func TestReadAndClassify_PrefixStripping(t *testing.T) {
	clearREPEnv(t)
	t.Setenv("REP_PUBLIC_FEATURE_FLAGS", "dark-mode,beta")

	vars, err := ReadAndClassify()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vars.Public) != 1 {
		t.Fatalf("expected 1 public var, got %d", len(vars.Public))
	}
	if vars.Public[0].Name != "FEATURE_FLAGS" {
		t.Errorf("prefix not stripped correctly: got %s", vars.Public[0].Name)
	}
}

func TestReadAndClassify_IgnoresGatewayVars(t *testing.T) {
	clearREPEnv(t)
	t.Setenv("REP_GATEWAY_PORT", "9090")
	t.Setenv("REP_GATEWAY_MODE", "embedded")

	vars, err := ReadAndClassify()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vars.Public)+len(vars.Sensitive)+len(vars.Server) != 0 {
		t.Errorf("gateway vars should not be classified, got %d total",
			len(vars.Public)+len(vars.Sensitive)+len(vars.Server))
	}
}

func TestReadAndClassify_IgnoresNonREPVars(t *testing.T) {
	clearREPEnv(t)
	// PATH and HOME should always be set but should be ignored.

	vars, err := ReadAndClassify()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vars.Public)+len(vars.Sensitive)+len(vars.Server) != 0 {
		t.Errorf("non-REP vars should be ignored, got %d total",
			len(vars.Public)+len(vars.Sensitive)+len(vars.Server))
	}
}

func TestReadAndClassify_NameCollision(t *testing.T) {
	clearREPEnv(t)
	t.Setenv("REP_PUBLIC_FOO", "a")
	t.Setenv("REP_SENSITIVE_FOO", "b")

	_, err := ReadAndClassify()
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
}

func TestReadAndClassify_IgnoresUnknownREPPrefix(t *testing.T) {
	clearREPEnv(t)
	t.Setenv("REP_CUSTOM_FOO", "bar")

	vars, err := ReadAndClassify()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vars.Public)+len(vars.Sensitive)+len(vars.Server) != 0 {
		t.Errorf("unknown REP prefix should be ignored, got %d total",
			len(vars.Public)+len(vars.Sensitive)+len(vars.Server))
	}
}

func TestReadAndClassify_MultipleVars(t *testing.T) {
	clearREPEnv(t)
	t.Setenv("REP_PUBLIC_API_URL", "https://api.example.com")
	t.Setenv("REP_PUBLIC_CDN_URL", "https://cdn.example.com")
	t.Setenv("REP_SENSITIVE_KEY", "secret")
	t.Setenv("REP_SERVER_DB", "dbpass")

	vars, err := ReadAndClassify()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vars.Public) != 2 {
		t.Errorf("expected 2 public vars, got %d", len(vars.Public))
	}
	if len(vars.Sensitive) != 1 {
		t.Errorf("expected 1 sensitive var, got %d", len(vars.Sensitive))
	}
	if len(vars.Server) != 1 {
		t.Errorf("expected 1 server var, got %d", len(vars.Server))
	}
}

func TestPublicMap(t *testing.T) {
	vars := &ClassifiedVars{
		Public: []Variable{
			{Name: "A", Value: "1"},
			{Name: "B", Value: "2"},
		},
	}

	m := vars.PublicMap()
	if m["A"] != "1" || m["B"] != "2" {
		t.Errorf("PublicMap returned %v", m)
	}
	if len(m) != 2 {
		t.Errorf("expected 2 entries, got %d", len(m))
	}
}

func TestSensitiveMap(t *testing.T) {
	vars := &ClassifiedVars{
		Sensitive: []Variable{
			{Name: "KEY", Value: "secret"},
		},
	}

	m := vars.SensitiveMap()
	if m["KEY"] != "secret" {
		t.Errorf("SensitiveMap returned %v", m)
	}
}

func TestTierString(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{TierPublic, "public"},
		{TierSensitive, "sensitive"},
		{TierServer, "server"},
		{Tier(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.want {
			t.Errorf("Tier(%d).String() = %q, want %q", tt.tier, got, tt.want)
		}
	}
}
