package guardrails

import (
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/ruachtech/rep/gateway/internal/config"
	"github.com/ruachtech/rep/gateway/internal/manifest"
)

func makeVars(publicVars ...config.Variable) *config.ClassifiedVars {
	return &config.ClassifiedVars{Public: publicVars}
}

func makeVar(name, value string) config.Variable {
	return config.Variable{
		Name:        name,
		Value:       value,
		Tier:        config.TierPublic,
		OriginalKey: "REP_PUBLIC_" + name,
	}
}

func TestScan_NoWarnings(t *testing.T) {
	vars := makeVars(
		makeVar("API_URL", "https://api.example.com"),
		makeVar("FEATURE_FLAGS", "dark-mode,beta"),
	)

	result := Scan(vars, nil, slog.Default())
	if result.HasWarnings() {
		t.Errorf("expected no warnings, got %d: %+v", len(result.Warnings), result.Warnings)
	}
}

func TestScan_KnownFormat_AWS(t *testing.T) {
	vars := makeVars(makeVar("KEY", "AKIAIOSFODNN7EXAMPLE"))

	result := Scan(vars, nil, slog.Default())
	if !result.HasWarnings() {
		t.Fatal("expected warning for AWS key format")
	}
	if result.Warnings[0].DetectionType != "known_format" {
		t.Errorf("expected known_format, got %s", result.Warnings[0].DetectionType)
	}
}

func TestScan_KnownFormat_JWT(t *testing.T) {
	vars := makeVars(makeVar("TOKEN", "eyJhbGciOiJIUzI1NiJ9.test.payload"))

	result := Scan(vars, nil, slog.Default())
	found := false
	for _, w := range result.Warnings {
		if w.DetectionType == "known_format" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected known_format warning for JWT")
	}
}

func TestScan_KnownFormat_GitHub(t *testing.T) {
	vars := makeVars(makeVar("GH", "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"))

	result := Scan(vars, nil, slog.Default())
	found := false
	for _, w := range result.Warnings {
		if w.DetectionType == "known_format" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected known_format warning for GitHub PAT")
	}
}

func TestScan_KnownFormat_Stripe(t *testing.T) {
	vars := makeVars(makeVar("SK", "sk_live_xxxxxxxxxxxxxxxxxxxxxxxx"))

	result := Scan(vars, nil, slog.Default())
	found := false
	for _, w := range result.Warnings {
		if w.DetectionType == "known_format" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected known_format warning for Stripe key")
	}
}

func TestScan_KnownFormat_OpenAI(t *testing.T) {
	vars := makeVars(makeVar("AI", "sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"))

	result := Scan(vars, nil, slog.Default())
	found := false
	for _, w := range result.Warnings {
		if w.DetectionType == "known_format" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected known_format warning for OpenAI key")
	}
}

func TestScan_KnownFormat_PrivateKey(t *testing.T) {
	vars := makeVars(makeVar("CERT", "-----BEGIN RSA PRIVATE KEY-----"))

	result := Scan(vars, nil, slog.Default())
	found := false
	for _, w := range result.Warnings {
		if w.DetectionType == "known_format" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected known_format warning for private key")
	}
}

func TestScan_HighEntropy(t *testing.T) {
	// A random-looking string with high entropy, >16 chars.
	highEntropy := "x9Kp2mQ7wL5rT0uN3jH8vB6yC1fZ4gD" // 32 unique chars, high entropy

	vars := makeVars(makeVar("RANDOM", highEntropy))

	result := Scan(vars, nil, slog.Default())
	found := false
	for _, w := range result.Warnings {
		if w.DetectionType == "high_entropy" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected high_entropy warning for random-looking string")
	}
}

func TestScan_LengthAnomaly(t *testing.T) {
	// 70-char string, no spaces, no http prefix.
	longValue := "abcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ12345678"

	vars := makeVars(makeVar("LONG", longValue))

	result := Scan(vars, nil, slog.Default())
	found := false
	for _, w := range result.Warnings {
		if w.DetectionType == "length_anomaly" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected length_anomaly warning")
	}
}

func TestScan_NoFalsePositive_URL(t *testing.T) {
	// Long URL should not trigger length_anomaly.
	longURL := "https://very-long-subdomain.example.com/path/to/resource/with/many/segments/that/exceed/sixty/four/characters"

	vars := makeVars(makeVar("CDN", longURL))

	result := Scan(vars, nil, slog.Default())
	for _, w := range result.Warnings {
		if w.DetectionType == "length_anomaly" {
			t.Error("URL should not trigger length_anomaly")
		}
	}
}

func TestScan_NoFalsePositive_ShortValue(t *testing.T) {
	// Short high-entropy string (under 16 chars) should not trigger.
	vars := makeVars(makeVar("SHORT", "aB3cD4eF5gH"))

	result := Scan(vars, nil, slog.Default())
	for _, w := range result.Warnings {
		if w.DetectionType == "high_entropy" {
			t.Error("short values should not trigger entropy warning")
		}
	}
}

func TestScanOnlyScansPublic(t *testing.T) {
	// Sensitive and server vars should not be scanned.
	vars := &config.ClassifiedVars{
		Public:    []config.Variable{},
		Sensitive: []config.Variable{{Name: "KEY", Value: "AKIAIOSFODNN7EXAMPLE", Tier: config.TierSensitive}},
		Server:    []config.Variable{{Name: "DB", Value: "sk_live_secret", Tier: config.TierServer}},
	}

	result := Scan(vars, nil, slog.Default())
	if result.HasWarnings() {
		t.Error("guardrails should only scan PUBLIC vars")
	}
}

func TestShannonEntropy_Empty(t *testing.T) {
	e := shannonEntropy("")
	if e != 0 {
		t.Errorf("expected 0 for empty string, got %f", e)
	}
}

func TestShannonEntropy_AllSame(t *testing.T) {
	e := shannonEntropy("aaaa")
	if e != 0 {
		t.Errorf("expected 0 for uniform string, got %f", e)
	}
}

func TestShannonEntropy_TwoChars(t *testing.T) {
	e := shannonEntropy("ab")
	if e < 0.99 || e > 1.01 {
		t.Errorf("expected ~1.0 for 'ab', got %f", e)
	}
}

func TestShannonEntropy_HighValue(t *testing.T) {
	// A string with many distinct characters should have high entropy.
	e := shannonEntropy("aB3cD4eF5gH6iJ7kL8mN9oP0")
	if e <= 4.0 {
		t.Errorf("expected high entropy for diverse string, got %f", e)
	}
}

func TestScan_CSVJudgedByElement(t *testing.T) {
	// 67 chars of short kebab-case flags: long, no spaces, no URL prefix.
	const flagList = "dark-mode,new-checkout,beta-search,lyrics-web,stage-timer,obs-scene"
	opaque := strings.Repeat("Zm9vYmFy", 10) // 80 chars, base64-looking
	declared := func(typ string) *manifest.Manifest {
		return &manifest.Manifest{Variables: map[string]*manifest.VarDecl{
			"FEATURE_FLAGS": {Tier: "public", Type: typ},
		}}
	}
	csv, str := declared("csv"), declared("string")

	tests := []struct {
		name     string
		manifest *manifest.Manifest
		value    string
		want     []string // detection types, in order
		wantMsg  string   // substring of the first warning's message
	}{
		{name: "long csv of short tokens passes", manifest: csv, value: flagList},
		{name: "spaces after commas are trimmed", manifest: csv, value: strings.ReplaceAll(flagList, ",", " , ")},
		{name: "empty csv passes", manifest: csv, value: ""},
		{
			name: "one long opaque token inside a csv is flagged", manifest: csv,
			value: "dark-mode," + opaque + ",beta",
			want:  []string{"length_anomaly"}, wantMsg: "csv element 2: value is 80 chars",
		},
		{
			name: "a known secret format past the first element is flagged", manifest: csv,
			value: "dark-mode,ghp_" + strings.Repeat("ab", 18),
			want:  []string{"known_format"}, wantMsg: "csv element 2: value matches known GitHub Personal Access Token",
		},
		{
			name: "a high-entropy element is flagged", manifest: csv,
			value: "beta,aB3$xY9!mK2@pQ7#nL5&wR8*",
			want:  []string{"high_entropy"}, wantMsg: "csv element 2: value has high entropy",
		},
		{
			name: "the same comma list typed string is still flagged whole", manifest: str,
			value: flagList,
			want:  []string{"length_anomaly"}, wantMsg: "value is 67 chars",
		},
		{
			name: "without a manifest a comma list is flagged whole", manifest: nil,
			value: flagList,
			want:  []string{"length_anomaly"}, wantMsg: "value is 67 chars",
		},
		{
			name: "a long opaque non-csv value is flagged", manifest: str,
			value: opaque,
			want:  []string{"length_anomaly"}, wantMsg: "value is 80 chars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Scan(makeVars(makeVar("FEATURE_FLAGS", tt.value)), tt.manifest, slog.Default())

			var got []string
			for _, w := range result.Warnings {
				got = append(got, w.DetectionType)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("detections = %v, want %v (%+v)", got, tt.want, result.Warnings)
			}
			if tt.wantMsg != "" && !strings.HasPrefix(result.Warnings[0].Message, tt.wantMsg) {
				t.Errorf("message = %q, want prefix %q", result.Warnings[0].Message, tt.wantMsg)
			}
		})
	}
}
