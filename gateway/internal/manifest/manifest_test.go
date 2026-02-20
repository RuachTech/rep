package manifest

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Parser tests
// ---------------------------------------------------------------------------

func TestParseMinimalManifest(t *testing.T) {
	lines := strings.Split(`version: "0.1.0"
variables:
  API_URL:
    tier: public
    type: url
    required: true
    description: "Base URL for the backend REST API"
`, "\n")

	m, err := parseManifest(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Version != "0.1.0" {
		t.Errorf("version: got %q, want %q", m.Version, "0.1.0")
	}
	decl, ok := m.Variables["API_URL"]
	if !ok {
		t.Fatal("API_URL not found in Variables")
	}
	if decl.Tier != "public" {
		t.Errorf("tier: got %q, want %q", decl.Tier, "public")
	}
	if decl.Type != "url" {
		t.Errorf("type: got %q, want %q", decl.Type, "url")
	}
	if !decl.Required {
		t.Error("required: expected true")
	}
}

func TestParseWithSettings(t *testing.T) {
	lines := strings.Split(`version: "0.1.0"
variables: {}
settings:
  strict_guardrails: true
  hot_reload: true
  hot_reload_mode: "poll"
  session_key_ttl: "60s"
  session_key_max_rate: 5
`, "\n")

	m, err := parseManifest(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Settings == nil {
		t.Fatal("settings block not parsed")
	}
	if !m.Settings.StrictGuardrails {
		t.Error("strict_guardrails: expected true")
	}
	if !m.Settings.HotReload {
		t.Error("hot_reload: expected true")
	}
	if m.Settings.HotReloadMode != "poll" {
		t.Errorf("hot_reload_mode: got %q, want %q", m.Settings.HotReloadMode, "poll")
	}
	if m.Settings.SessionKeyTTL != 60*time.Second {
		t.Errorf("session_key_ttl: got %v, want 60s", m.Settings.SessionKeyTTL)
	}
	if m.Settings.SessionKeyMaxRate != 5 {
		t.Errorf("session_key_max_rate: got %d, want 5", m.Settings.SessionKeyMaxRate)
	}
}

func TestParseInlineEnumValues(t *testing.T) {
	lines := strings.Split(`version: "0.1.0"
variables:
  ENV_NAME:
    tier: public
    type: enum
    required: true
    values: ["development", "staging", "production"]
`, "\n")

	m, err := parseManifest(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decl := m.Variables["ENV_NAME"]
	if decl == nil {
		t.Fatal("ENV_NAME not parsed")
	}
	if len(decl.Values) != 3 {
		t.Fatalf("values: got %d items, want 3", len(decl.Values))
	}
	want := []string{"development", "staging", "production"}
	for i, w := range want {
		if decl.Values[i] != w {
			t.Errorf("values[%d]: got %q, want %q", i, decl.Values[i], w)
		}
	}
}

func TestParseMultiLineEnumValues(t *testing.T) {
	lines := strings.Split(`version: "0.1.0"
variables:
  ENV_NAME:
    tier: public
    type: enum
    values:
      - development
      - staging
      - production
    required: false
`, "\n")

	m, err := parseManifest(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decl := m.Variables["ENV_NAME"]
	if decl == nil {
		t.Fatal("ENV_NAME not parsed")
	}
	if len(decl.Values) != 3 {
		t.Fatalf("values: got %d items, want 3: %v", len(decl.Values), decl.Values)
	}
}

func TestParseMultipleVariables(t *testing.T) {
	lines := strings.Split(`version: "0.1.0"
variables:
  API_URL:
    tier: public
    type: url
    required: true
  ANALYTICS_KEY:
    tier: sensitive
    type: string
    required: true
  DB_PASSWORD:
    tier: server
    type: string
    required: false
`, "\n")

	m, err := parseManifest(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Variables) != 3 {
		t.Fatalf("variable count: got %d, want 3", len(m.Variables))
	}
	if m.Variables["API_URL"].Tier != "public" {
		t.Errorf("API_URL tier: got %q", m.Variables["API_URL"].Tier)
	}
	if m.Variables["ANALYTICS_KEY"].Tier != "sensitive" {
		t.Errorf("ANALYTICS_KEY tier: got %q", m.Variables["ANALYTICS_KEY"].Tier)
	}
	if m.Variables["DB_PASSWORD"].Tier != "server" {
		t.Errorf("DB_PASSWORD tier: got %q", m.Variables["DB_PASSWORD"].Tier)
	}
}

func TestParseWithPattern(t *testing.T) {
	lines := strings.Split(`version: "0.1.0"
variables:
  OAUTH_CLIENT_ID:
    tier: sensitive
    type: string
    required: true
    pattern: "^[a-zA-Z0-9]{20,}$"
`, "\n")

	m, err := parseManifest(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decl := m.Variables["OAUTH_CLIENT_ID"]
	if decl == nil {
		t.Fatal("OAUTH_CLIENT_ID not parsed")
	}
	if decl.Pattern != "^[a-zA-Z0-9]{20,}$" {
		t.Errorf("pattern: got %q", decl.Pattern)
	}
}

func TestParseDeprecated(t *testing.T) {
	lines := strings.Split(`version: "0.1.0"
variables:
  OLD_API_KEY:
    tier: public
    type: string
    required: false
    deprecated: true
    deprecated_message: "Use ANALYTICS_KEY instead"
`, "\n")

	m, err := parseManifest(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decl := m.Variables["OLD_API_KEY"]
	if decl == nil {
		t.Fatal("OLD_API_KEY not parsed")
	}
	if !decl.Deprecated {
		t.Error("deprecated: expected true")
	}
	if decl.DeprecatedMessage != "Use ANALYTICS_KEY instead" {
		t.Errorf("deprecated_message: got %q", decl.DeprecatedMessage)
	}
}

func TestParseComments(t *testing.T) {
	lines := strings.Split(`# Top-level comment
version: "0.1.0"  # inline comment
variables:  # another comment
  API_URL:  # variable comment
    tier: public  # tier comment
    type: url
    required: true
`, "\n")

	m, err := parseManifest(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Version != "0.1.0" {
		t.Errorf("version: got %q", m.Version)
	}
	if m.Variables["API_URL"] == nil {
		t.Error("API_URL not found after comment stripping")
	}
}

func TestParseHotReloadModeSettings(t *testing.T) {
	lines := strings.Split(`version: "0.1.0"
variables: {}
settings:
  hot_reload: true
  hot_reload_mode: "file_watch"
  hot_reload_poll_interval: "1m"
`, "\n")

	m, err := parseManifest(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Settings.HotReloadMode != "file_watch" {
		t.Errorf("hot_reload_mode: got %q", m.Settings.HotReloadMode)
	}
	if m.Settings.HotReloadPollInterval != time.Minute {
		t.Errorf("hot_reload_poll_interval: got %v, want 1m", m.Settings.HotReloadPollInterval)
	}
}

func TestParseAllowedOriginsInline(t *testing.T) {
	lines := strings.Split(`version: "0.1.0"
variables: {}
settings:
  allowed_origins: ["https://app.example.com", "https://admin.example.com"]
`, "\n")

	m, err := parseManifest(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Settings.AllowedOrigins) != 2 {
		t.Fatalf("allowed_origins count: got %d, want 2", len(m.Settings.AllowedOrigins))
	}
}

func TestParseDefaultValue(t *testing.T) {
	lines := strings.Split(`version: "0.1.0"
variables:
  FEATURE_FLAGS:
    tier: public
    type: csv
    required: false
    default: ""
`, "\n")

	m, err := parseManifest(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decl := m.Variables["FEATURE_FLAGS"]
	if !decl.HasDefault {
		t.Error("HasDefault: expected true for explicit empty default")
	}
}

// ---------------------------------------------------------------------------
// Validation tests
// ---------------------------------------------------------------------------

func TestValidateRequiredPresent(t *testing.T) {
	m := &Manifest{
		Variables: map[string]*VarDecl{
			"API_URL": {Tier: "public", Type: "url", Required: true},
		},
	}
	public := map[string]string{"API_URL": "https://api.example.com"}
	if err := m.Validate(public, nil, nil, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateRequiredMissing(t *testing.T) {
	m := &Manifest{
		Variables: map[string]*VarDecl{
			"API_URL": {Tier: "public", Type: "url", Required: true},
		},
	}
	if err := m.Validate(nil, nil, nil, nil); err == nil {
		t.Error("expected error for missing required variable")
	}
}

func TestValidateTypeURL(t *testing.T) {
	m := &Manifest{
		Variables: map[string]*VarDecl{
			"API_URL": {Tier: "public", Type: "url", Required: true},
		},
	}
	public := map[string]string{"API_URL": "not-a-url"}
	if err := m.Validate(public, nil, nil, nil); err == nil {
		t.Error("expected error for invalid URL")
	}
	// Valid URL should pass.
	public["API_URL"] = "https://api.example.com"
	if err := m.Validate(public, nil, nil, nil); err != nil {
		t.Errorf("unexpected error for valid URL: %v", err)
	}
}

func TestValidateTypeNumber(t *testing.T) {
	m := &Manifest{
		Variables: map[string]*VarDecl{
			"RATE": {Tier: "server", Type: "number", Required: true},
		},
	}
	if err := m.Validate(nil, nil, map[string]string{"RATE": "abc"}, nil); err == nil {
		t.Error("expected error for non-number")
	}
	if err := m.Validate(nil, nil, map[string]string{"RATE": "42.5"}, nil); err != nil {
		t.Errorf("unexpected error for valid number: %v", err)
	}
}

func TestValidateTypeBoolean(t *testing.T) {
	m := &Manifest{
		Variables: map[string]*VarDecl{
			"FLAG": {Tier: "public", Type: "boolean", Required: true},
		},
	}
	for _, bad := range []string{"yes", "enabled", "on"} {
		if err := m.Validate(map[string]string{"FLAG": bad}, nil, nil, nil); err == nil {
			t.Errorf("expected error for boolean value %q", bad)
		}
	}
	for _, good := range []string{"true", "false", "1", "0", "True", "FALSE"} {
		if err := m.Validate(map[string]string{"FLAG": good}, nil, nil, nil); err != nil {
			t.Errorf("unexpected error for boolean value %q: %v", good, err)
		}
	}
}

func TestValidateTypeEnum(t *testing.T) {
	m := &Manifest{
		Variables: map[string]*VarDecl{
			"ENV": {
				Tier:     "public",
				Type:     "enum",
				Required: true,
				Values:   []string{"development", "staging", "production"},
			},
		},
	}
	if err := m.Validate(map[string]string{"ENV": "unknown"}, nil, nil, nil); err == nil {
		t.Error("expected error for invalid enum value")
	}
	if err := m.Validate(map[string]string{"ENV": "staging"}, nil, nil, nil); err != nil {
		t.Errorf("unexpected error for valid enum value: %v", err)
	}
}

func TestValidatePattern(t *testing.T) {
	m := &Manifest{
		Variables: map[string]*VarDecl{
			"CLIENT_ID": {
				Tier:     "sensitive",
				Type:     "string",
				Required: true,
				Pattern:  `[a-zA-Z0-9]{20,}`,
			},
		},
	}
	if err := m.Validate(nil, map[string]string{"CLIENT_ID": "short"}, nil, nil); err == nil {
		t.Error("expected error for pattern mismatch")
	}
	if err := m.Validate(nil, map[string]string{"CLIENT_ID": "abcdefghij0123456789"}, nil, nil); err != nil {
		t.Errorf("unexpected error for pattern match: %v", err)
	}
}

func TestValidateDeprecatedWarning(t *testing.T) {
	m := &Manifest{
		Variables: map[string]*VarDecl{
			"OLD_KEY": {
				Tier:              "public",
				Type:              "string",
				Deprecated:        true,
				DeprecatedMessage: "Use NEW_KEY",
			},
		},
	}
	warned := false
	logFn := func(msg string, args ...any) {
		warned = true
	}
	// Deprecated variable present → warning, NOT an error.
	if err := m.Validate(map[string]string{"OLD_KEY": "value"}, nil, nil, logFn); err != nil {
		t.Errorf("unexpected error for deprecated var: %v", err)
	}
	if !warned {
		t.Error("expected log warning for deprecated variable")
	}
}

func TestValidateNilManifest(t *testing.T) {
	var m *Manifest
	if err := m.Validate(nil, nil, nil, nil); err != nil {
		t.Errorf("nil manifest should pass validation: %v", err)
	}
}

func TestValidateOptionalAbsent(t *testing.T) {
	m := &Manifest{
		Variables: map[string]*VarDecl{
			"OPTIONAL": {Tier: "public", Type: "string", Required: false},
		},
	}
	// Optional and absent → no error.
	if err := m.Validate(nil, nil, nil, nil); err != nil {
		t.Errorf("optional absent var should not error: %v", err)
	}
}

func TestLoadExampleManifest(t *testing.T) {
	// Load the actual example manifest from the repo.
	m, err := Load("../../../examples/.rep.yaml")
	if err != nil {
		t.Fatalf("failed to load example manifest: %v", err)
	}
	if m.Version == "" {
		t.Error("version should be set in example manifest")
	}
	if len(m.Variables) == 0 {
		t.Error("example manifest should declare variables")
	}
	// Check a known variable.
	apiURL, ok := m.Variables["API_URL"]
	if !ok {
		t.Error("API_URL should be in example manifest variables")
	}
	if apiURL.Tier != "public" {
		t.Errorf("API_URL tier: got %q, want public", apiURL.Tier)
	}
	if apiURL.Type != "url" {
		t.Errorf("API_URL type: got %q, want url", apiURL.Type)
	}
	if !apiURL.Required {
		t.Error("API_URL should be required in example manifest")
	}
	// Settings block.
	if m.Settings == nil {
		t.Error("example manifest should have a settings block")
	}
	if !m.Settings.HotReload {
		t.Error("example manifest settings.hot_reload should be true")
	}
}

// ---------------------------------------------------------------------------
// Helper tests
// ---------------------------------------------------------------------------

func TestUnquoteYAML(t *testing.T) {
	cases := []struct{ in, want string }{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{`hello`, "hello"},
		{`"a \"quoted\" word"`, `a "quoted" word`},
		{`""`, ""},
	}
	for _, tc := range cases {
		got := unquoteYAML(tc.in)
		if got != tc.want {
			t.Errorf("unquoteYAML(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseInlineSequence(t *testing.T) {
	got := parseInlineSequence(`["development", "staging", "production"]`)
	want := []string{"development", "staging", "production"}
	if len(got) != len(want) {
		t.Fatalf("length: got %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d]: got %q, want %q", i, got[i], w)
		}
	}
}

func TestStripComment(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  tier: public  # comment", "  tier: public  "},
		{`  url: "https://ex.com/#anchor"  # note`, `  url: "https://ex.com/#anchor"  `},
		{"no comment", "no comment"},
	}
	for _, tc := range cases {
		got := stripComment(tc.in)
		if got != tc.want {
			t.Errorf("stripComment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
