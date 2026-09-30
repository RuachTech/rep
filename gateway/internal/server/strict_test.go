package server

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/ruachtech/rep/gateway/internal/config"
	"github.com/ruachtech/rep/gateway/internal/manifest"
)

// flagList is 67 characters of short kebab-case flags joined by commas.
// Case-by-case coverage of the heuristics is in the guardrails package; this
// test proves the manifest reaches the scan and decides a --strict start.
const flagList = "dark-mode,new-checkout,beta-search,lyrics-web,stage-timer,obs-scene"

func TestNew_StrictGuardrailsJudgeCSVByElement(t *testing.T) {
	if len(flagList) <= 64 {
		t.Fatalf("flagList is %d chars; it must exceed the 64-char length heuristic", len(flagList))
	}
	opaque := strings.Repeat("Zm9vYmFy", 10) // 80 chars, base64-looking

	tests := []struct {
		name    string
		typ     string
		value   string
		wantErr bool
	}{
		{name: "long csv of short tokens starts", typ: "csv", value: flagList},
		{name: "csv holding one long opaque token is refused", typ: "csv", value: "dark-mode," + opaque + ",beta", wantErr: true},
		{name: "the same comma list typed string is still refused", typ: "string", value: flagList, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearREPEnv(t)
			t.Setenv("REP_PUBLIC_FEATURE_FLAGS", tt.value)

			cfg := &config.Config{
				Mode:      "embedded",
				StaticDir: "../../testdata/static",
				Strict:    true,
				Manifest: &manifest.Manifest{Variables: map[string]*manifest.VarDecl{
					"FEATURE_FLAGS": {Tier: "public", Type: tt.typ},
				}},
			}
			_, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "0.0.0-test")
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "refusing to start") {
					t.Fatalf("New() error = %v, want a strict-mode refusal", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("New() error = %v, want the gateway to start", err)
			}
		})
	}
}
