// Package config handles parsing of CLI flags, environment variables,
// and optional .rep.yaml manifest files for the REP gateway.
//
// Precedence (highest to lowest):
//  1. Command-line flags
//  2. REP_GATEWAY_* environment variables
//  3. .rep.yaml manifest settings
//  4. Defaults
package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the parsed gateway configuration.
type Config struct {
	// Operating mode: "proxy" or "embedded".
	Mode string

	// Upstream server address (proxy mode only).
	Upstream string

	// Listen port for the main server.
	Port int

	// Static file directory (embedded mode only).
	StaticDir string

	// Path to .rep.yaml manifest file.
	ManifestPath string

	// If true, guardrail warnings cause a startup failure.
	Strict bool

	// Hot reload configuration.
	HotReload     bool
	HotReloadMode string // "file_watch", "signal", "poll"
	WatchPath     string
	PollInterval  time.Duration

	// Logging.
	LogFormat    string // "json" or "text"
	LogLevelStr  string // "debug", "info", "warn", "error"

	// CORS allowed origins for /rep/session-key.
	AllowedOrigins []string

	// TLS (optional).
	TLSCert string
	TLSKey  string

	// Separate health check port (optional, for K8s probes).
	HealthPort int

	// Session key settings.
	SessionKeyTTL     time.Duration
	SessionKeyMaxRate int // Per minute per IP.

	// Version flag.
	ShowVersion bool
}

// LogLevel returns the slog.Level corresponding to the configured log level string.
func (c *Config) LogLevel() slog.Level {
	switch strings.ToLower(c.LogLevelStr) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Parse reads configuration from CLI flags and environment variables.
// CLI flags take precedence over environment variables.
func Parse(args []string, version string) (*Config, error) {
	cfg := &Config{}
	fs := flag.NewFlagSet("rep-gateway", flag.ContinueOnError)

	// Register flags.
	fs.StringVar(&cfg.Mode, "mode", envOrDefault("REP_GATEWAY_MODE", "proxy"), `Operating mode: "proxy" or "embedded"`)
	fs.StringVar(&cfg.Upstream, "upstream", envOrDefault("REP_GATEWAY_UPSTREAM", "localhost:80"), "Upstream server address (proxy mode)")
	fs.IntVar(&cfg.Port, "port", envOrDefaultInt("REP_GATEWAY_PORT", 8080), "Listen port")
	fs.StringVar(&cfg.StaticDir, "static-dir", envOrDefault("REP_GATEWAY_STATIC_DIR", "/usr/share/nginx/html"), "Static file directory (embedded mode)")
	// TODO(manifest): ManifestPath is stored but never read. Implement YAML
	// parsing, required-variable validation, type checking, and settings
	// overlay (manifest < env < flag precedence) per REP-RFC-0001 §6.
	fs.StringVar(&cfg.ManifestPath, "manifest", envOrDefault("REP_GATEWAY_MANIFEST", ""), "Path to .rep.yaml manifest")
	fs.BoolVar(&cfg.Strict, "strict", envOrDefaultBool("REP_GATEWAY_STRICT", false), "Exit on guardrail warnings")
	fs.BoolVar(&cfg.HotReload, "hot-reload", envOrDefaultBool("REP_GATEWAY_HOT_RELOAD", false), "Enable hot reload SSE endpoint")
	fs.StringVar(&cfg.HotReloadMode, "hot-reload-mode", envOrDefault("REP_GATEWAY_HOT_RELOAD_MODE", "signal"), `Hot reload mode: "file_watch", "signal", "poll"`)
	fs.StringVar(&cfg.WatchPath, "watch-path", envOrDefault("REP_GATEWAY_WATCH_PATH", ""), "Path to watch for config changes (file_watch mode)")
	pollInterval := fs.String("poll-interval", envOrDefault("REP_GATEWAY_POLL_INTERVAL", "30s"), "Poll interval (poll mode)")
	fs.StringVar(&cfg.LogFormat, "log-format", envOrDefault("REP_GATEWAY_LOG_FORMAT", "json"), `Log format: "json" or "text"`)
	fs.StringVar(&cfg.LogLevelStr, "log-level", envOrDefault("REP_GATEWAY_LOG_LEVEL", "info"), `Log level: "debug", "info", "warn", "error"`)
	originsStr := fs.String("allowed-origins", envOrDefault("REP_GATEWAY_ALLOWED_ORIGINS", ""), "Comma-separated allowed CORS origins for /rep/session-key")
	fs.StringVar(&cfg.TLSCert, "tls-cert", envOrDefault("REP_GATEWAY_TLS_CERT", ""), "TLS certificate path")
	fs.StringVar(&cfg.TLSKey, "tls-key", envOrDefault("REP_GATEWAY_TLS_KEY", ""), "TLS private key path")
	fs.IntVar(&cfg.HealthPort, "health-port", envOrDefaultInt("REP_GATEWAY_HEALTH_PORT", 0), "Separate health check port (0 = same as main)")
	sessionTTL := fs.String("session-key-ttl", envOrDefault("REP_GATEWAY_SESSION_KEY_TTL", "30s"), "Session key TTL")
	fs.IntVar(&cfg.SessionKeyMaxRate, "session-key-max-rate", envOrDefaultInt("REP_GATEWAY_SESSION_KEY_MAX_RATE", 10), "Session key max requests/min/IP")
	fs.BoolVar(&cfg.ShowVersion, "version", false, "Print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// Parse durations.
	var err error
	cfg.PollInterval, err = time.ParseDuration(*pollInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid poll-interval %q: %w", *pollInterval, err)
	}
	cfg.SessionKeyTTL, err = time.ParseDuration(*sessionTTL)
	if err != nil {
		return nil, fmt.Errorf("invalid session-key-ttl %q: %w", *sessionTTL, err)
	}

	// Parse origins.
	if *originsStr != "" {
		cfg.AllowedOrigins = strings.Split(*originsStr, ",")
		for i := range cfg.AllowedOrigins {
			cfg.AllowedOrigins[i] = strings.TrimSpace(cfg.AllowedOrigins[i])
		}
	}

	// Validate mode.
	if cfg.Mode != "proxy" && cfg.Mode != "embedded" {
		return nil, fmt.Errorf("invalid mode %q: must be \"proxy\" or \"embedded\"", cfg.Mode)
	}

	// Validate hot reload mode.
	switch cfg.HotReloadMode {
	case "file_watch", "signal", "poll":
		// OK.
	default:
		return nil, fmt.Errorf("invalid hot-reload-mode %q: must be \"file_watch\", \"signal\", or \"poll\"", cfg.HotReloadMode)
	}

	return cfg, nil
}

// envOrDefault returns the value of the environment variable or the default.
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// envOrDefaultInt returns the int value of the environment variable or the default.
func envOrDefaultInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

// envOrDefaultBool returns the bool value of the environment variable or the default.
func envOrDefaultBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}
