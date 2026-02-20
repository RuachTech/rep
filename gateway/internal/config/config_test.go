package config

import (
	"log/slog"
	"testing"
)

func TestParse_Defaults(t *testing.T) {
	cfg, err := Parse([]string{}, "0.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Mode != "proxy" {
		t.Errorf("expected mode=proxy, got %s", cfg.Mode)
	}
	if cfg.Port != 8080 {
		t.Errorf("expected port=8080, got %d", cfg.Port)
	}
	if cfg.Strict {
		t.Error("expected strict=false by default")
	}
	if cfg.HotReload {
		t.Error("expected hot-reload=false by default")
	}
	if cfg.HotReloadMode != "signal" {
		t.Errorf("expected hot-reload-mode=signal, got %s", cfg.HotReloadMode)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("expected log-format=json, got %s", cfg.LogFormat)
	}
	if cfg.SessionKeyMaxRate != 10 {
		t.Errorf("expected session-key-max-rate=10, got %d", cfg.SessionKeyMaxRate)
	}
}

func TestParse_FlagOverrides(t *testing.T) {
	cfg, err := Parse([]string{"--mode", "embedded", "--port", "9090", "--strict"}, "0.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Mode != "embedded" {
		t.Errorf("expected mode=embedded, got %s", cfg.Mode)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port=9090, got %d", cfg.Port)
	}
	if !cfg.Strict {
		t.Error("expected strict=true")
	}
}

func TestParse_EnvOverrides(t *testing.T) {
	t.Setenv("REP_GATEWAY_MODE", "embedded")
	t.Setenv("REP_GATEWAY_PORT", "3000")

	cfg, err := Parse([]string{}, "0.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Mode != "embedded" {
		t.Errorf("expected mode=embedded from env, got %s", cfg.Mode)
	}
	if cfg.Port != 3000 {
		t.Errorf("expected port=3000 from env, got %d", cfg.Port)
	}
}

func TestParse_FlagPrecedence(t *testing.T) {
	t.Setenv("REP_GATEWAY_MODE", "embedded")

	cfg, err := Parse([]string{"--mode", "proxy"}, "0.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Mode != "proxy" {
		t.Errorf("flag should override env: expected mode=proxy, got %s", cfg.Mode)
	}
}

func TestParse_InvalidMode(t *testing.T) {
	_, err := Parse([]string{"--mode", "invalid"}, "0.1.0")
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestParse_InvalidHotReloadMode(t *testing.T) {
	_, err := Parse([]string{"--hot-reload-mode", "invalid"}, "0.1.0")
	if err == nil {
		t.Fatal("expected error for invalid hot-reload-mode")
	}
}

func TestParse_AllowedOrigins(t *testing.T) {
	cfg, err := Parse([]string{"--allowed-origins", "https://a.com, https://b.com"}, "0.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("expected 2 origins, got %d", len(cfg.AllowedOrigins))
	}
	if cfg.AllowedOrigins[0] != "https://a.com" {
		t.Errorf("expected first origin https://a.com, got %s", cfg.AllowedOrigins[0])
	}
	if cfg.AllowedOrigins[1] != "https://b.com" {
		t.Errorf("expected second origin https://b.com, got %s", cfg.AllowedOrigins[1])
	}
}

func TestParse_VersionFlag(t *testing.T) {
	cfg, err := Parse([]string{"--version"}, "0.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.ShowVersion {
		t.Error("expected ShowVersion=true")
	}
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		level string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},        // Default
		{"unknown", slog.LevelInfo}, // Default
	}

	for _, tt := range tests {
		cfg := &Config{LogLevelStr: tt.level}
		if got := cfg.LogLevel(); got != tt.want {
			t.Errorf("LogLevel(%q) = %v, want %v", tt.level, got, tt.want)
		}
	}
}
