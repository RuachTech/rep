package payload

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ruach-tech/rep/gateway/internal/config"
	repcrypto "github.com/ruach-tech/rep/gateway/internal/crypto"
)

func testKeys(t *testing.T) *repcrypto.Keys {
	t.Helper()
	keys, err := repcrypto.GenerateKeys()
	if err != nil {
		t.Fatalf("generating keys: %v", err)
	}
	return keys
}

func TestBuild_PublicOnly(t *testing.T) {
	keys := testKeys(t)
	builder := NewBuilder(keys, "0.1.0", false)

	vars := &config.ClassifiedVars{
		Public: []config.Variable{
			{Name: "API_URL", Value: "https://api.example.com"},
		},
	}

	p, err := builder.Build(vars)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	if p.Public["API_URL"] != "https://api.example.com" {
		t.Errorf("expected API_URL, got %v", p.Public)
	}
	if p.Sensitive != "" {
		t.Error("expected empty sensitive blob with no sensitive vars")
	}
	if p.Meta.KeyEndpoint != "" {
		t.Error("expected no key_endpoint without sensitive vars")
	}
}

func TestBuild_WithSensitive(t *testing.T) {
	keys := testKeys(t)
	builder := NewBuilder(keys, "0.1.0", false)

	vars := &config.ClassifiedVars{
		Public: []config.Variable{
			{Name: "API_URL", Value: "https://api.example.com"},
		},
		Sensitive: []config.Variable{
			{Name: "ANALYTICS_KEY", Value: "UA-12345"},
		},
	}

	p, err := builder.Build(vars)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	if p.Sensitive == "" {
		t.Error("expected non-empty sensitive blob")
	}
	if p.Meta.KeyEndpoint != "/rep/session-key" {
		t.Errorf("expected key_endpoint=/rep/session-key, got %s", p.Meta.KeyEndpoint)
	}
}

func TestBuild_HotReload(t *testing.T) {
	keys := testKeys(t)
	builder := NewBuilder(keys, "0.1.0", true)

	vars := &config.ClassifiedVars{}

	p, err := builder.Build(vars)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	if p.Meta.HotReload != "/rep/changes" {
		t.Errorf("expected hot_reload=/rep/changes, got %s", p.Meta.HotReload)
	}
}

func TestBuild_NoHotReload(t *testing.T) {
	keys := testKeys(t)
	builder := NewBuilder(keys, "0.1.0", false)

	p, err := builder.Build(&config.ClassifiedVars{})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	if p.Meta.HotReload != "" {
		t.Errorf("expected empty hot_reload, got %s", p.Meta.HotReload)
	}
}

func TestBuild_IntegrityFormat(t *testing.T) {
	keys := testKeys(t)
	builder := NewBuilder(keys, "0.1.0", false)

	p, err := builder.Build(&config.ClassifiedVars{
		Public: []config.Variable{{Name: "K", Value: "V"}},
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	if !strings.HasPrefix(p.Meta.Integrity, "hmac-sha256:") {
		t.Errorf("expected hmac-sha256: prefix, got %q", p.Meta.Integrity)
	}
}

func TestBuild_InjectedAt(t *testing.T) {
	keys := testKeys(t)
	builder := NewBuilder(keys, "0.1.0", false)

	p, err := builder.Build(&config.ClassifiedVars{})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	_, err = time.Parse(time.RFC3339Nano, p.Meta.InjectedAt)
	if err != nil {
		t.Errorf("injected_at is not valid RFC3339Nano: %v", err)
	}
}

func TestScriptTag_Format(t *testing.T) {
	keys := testKeys(t)
	builder := NewBuilder(keys, "0.1.0", false)

	p, err := builder.Build(&config.ClassifiedVars{
		Public: []config.Variable{{Name: "X", Value: "1"}},
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	tag, err := p.ScriptTag()
	if err != nil {
		t.Fatalf("ScriptTag error: %v", err)
	}

	if !strings.Contains(tag, `id="__rep__"`) {
		t.Error("expected id=\"__rep__\" in script tag")
	}
	if !strings.Contains(tag, `type="application/json"`) {
		t.Error("expected type=\"application/json\" in script tag")
	}
	if !strings.Contains(tag, `data-rep-version="0.1.0"`) {
		t.Error("expected data-rep-version in script tag")
	}
	if !strings.Contains(tag, `data-rep-integrity="sha256-`) {
		t.Error("expected data-rep-integrity in script tag")
	}
}

func TestScriptTag_ValidJSON(t *testing.T) {
	keys := testKeys(t)
	builder := NewBuilder(keys, "0.1.0", false)

	p, err := builder.Build(&config.ClassifiedVars{
		Public: []config.Variable{{Name: "URL", Value: "https://example.com"}},
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	tag, err := p.ScriptTag()
	if err != nil {
		t.Fatalf("ScriptTag error: %v", err)
	}

	// Extract JSON from the script tag.
	start := strings.Index(tag, ">") + 1
	end := strings.Index(tag, "</script>")
	jsonStr := tag[start:end]

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("script tag content is not valid JSON: %v", err)
	}

	if _, ok := parsed["public"]; !ok {
		t.Error("expected 'public' key in parsed JSON")
	}
	if _, ok := parsed["_meta"]; !ok {
		t.Error("expected '_meta' key in parsed JSON")
	}
}

func TestToJSON_Roundtrip(t *testing.T) {
	keys := testKeys(t)
	builder := NewBuilder(keys, "0.1.0", true)

	p, err := builder.Build(&config.ClassifiedVars{
		Public: []config.Variable{{Name: "A", Value: "1"}},
	})
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	jsonBytes, err := p.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	meta, ok := parsed["_meta"].(map[string]interface{})
	if !ok {
		t.Fatal("_meta should be an object")
	}
	if meta["version"] != "0.1.0" {
		t.Errorf("expected version=0.1.0, got %v", meta["version"])
	}
}
