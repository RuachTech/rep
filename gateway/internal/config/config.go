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

	"github.com/ruachtech/rep/gateway/internal/manifest"
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
	LogFormat   string // "json" or "text"
	LogLevelStr string // "debug", "info", "warn", "error"

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

	// Loaded manifest (nil if --manifest was not specified or load failed).
	Manifest *manifest.Manifest
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
// Precedence (highest to lowest): CLI flags > REP_GATEWAY_* env vars > .rep.yaml settings > defaults.
func Parse(args []string, version string) (*Config, error) {
	cfg := &Config{}

	// ── Phase 1: Pre-scan for --manifest so we can seed flag defaults from it ──
	manifestPath := prescanManifestFlag(args)
	if manifestPath == "" {
		manifestPath = os.Getenv("REP_GATEWAY_MANIFEST")
	}
	cfg.ManifestPath = manifestPath
	if manifestPath != "" {
		m, err := manifest.Load(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("loading manifest: %w", err)
		}
		cfg.Manifest = m
	}

	// ── Phase 2: Compute manifest-derived defaults for settings ──────────────
	// These are overridden by REP_GATEWAY_* env vars and CLI flags below.
	defaultHotReload := false
	defaultHotReloadMode := "signal"
	defaultPollInterval := "30s"
	defaultSessionTTL := "30s"
	defaultSessionMaxRate := 10
	defaultStrict := false
	var defaultAllowedOrigins string

	if m := cfg.Manifest; m != nil && m.Settings != nil {
		defaultHotReload = m.Settings.HotReload
		if m.Settings.HotReloadMode != "" {
			defaultHotReloadMode = m.Settings.HotReloadMode
		}
		if m.Settings.HotReloadPollInterval > 0 {
			defaultPollInterval = m.Settings.HotReloadPollInterval.String()
		}
		if m.Settings.SessionKeyTTL > 0 {
			defaultSessionTTL = m.Settings.SessionKeyTTL.String()
		}
		if m.Settings.SessionKeyMaxRate > 0 {
			defaultSessionMaxRate = m.Settings.SessionKeyMaxRate
		}
		defaultStrict = m.Settings.StrictGuardrails
		if len(m.Settings.AllowedOrigins) > 0 {
			defaultAllowedOrigins = strings.Join(m.Settings.AllowedOrigins, ",")
		}
	}

	// ── Phase 3: Parse flags (env vars overlay manifest, CLI flags overlay both)
	fs := flag.NewFlagSet("rep-gateway", flag.ContinueOnError)

	// Register flags.
	fs.StringVar(&cfg.Mode, "mode", envOrDefault("REP_GATEWAY_MODE", "proxy"), `Operating mode: "proxy" or "embedded"`)
	fs.StringVar(&cfg.Upstream, "upstream", envOrDefault("REP_GATEWAY_UPSTREAM", "localhost:80"), "Upstream server address (proxy mode)")
	fs.IntVar(&cfg.Port, "port", envOrDefaultInt("REP_GATEWAY_PORT", 8080), "Listen port")
	fs.StringVar(&cfg.StaticDir, "static-dir", envOrDefault("REP_GATEWAY_STATIC_DIR", "/usr/share/nginx/html"), "Static file directory (embedded mode)")
	fs.StringVar(&cfg.ManifestPath, "manifest", envOrDefault("REP_GATEWAY_MANIFEST", manifestPath), "Path to .rep.yaml manifest")
	fs.BoolVar(&cfg.Strict, "strict", envOrDefaultBool("REP_GATEWAY_STRICT", defaultStrict), "Exit on guardrail warnings")
	fs.BoolVar(&cfg.HotReload, "hot-reload", envOrDefaultBool("REP_GATEWAY_HOT_RELOAD", defaultHotReload), "Enable hot reload SSE endpoint")
	fs.StringVar(&cfg.HotReloadMode, "hot-reload-mode", envOrDefault("REP_GATEWAY_HOT_RELOAD_MODE", defaultHotReloadMode), `Hot reload mode: "file_watch", "signal", or "poll"`)
	fs.StringVar(&cfg.WatchPath, "watch-path", envOrDefault("REP_GATEWAY_WATCH_PATH", ""), "Path to watch for config changes (file_watch mode)")
	pollInterval := fs.String("poll-interval", envOrDefault("REP_GATEWAY_POLL_INTERVAL", defaultPollInterval), "Poll interval (poll mode)")
	fs.StringVar(&cfg.LogFormat, "log-format", envOrDefault("REP_GATEWAY_LOG_FORMAT", "json"), `Log format: "json" or "text"`)
	fs.StringVar(&cfg.LogLevelStr, "log-level", envOrDefault("REP_GATEWAY_LOG_LEVEL", "info"), `Log level: "debug", "info", "warn", "error"`)
	originsStr := fs.String("allowed-origins", envOrDefault("REP_GATEWAY_ALLOWED_ORIGINS", defaultAllowedOrigins), "Comma-separated allowed CORS origins for /rep/session-key")
	fs.StringVar(&cfg.TLSCert, "tls-cert", envOrDefault("REP_GATEWAY_TLS_CERT", ""), "TLS certificate path")
	fs.StringVar(&cfg.TLSKey, "tls-key", envOrDefault("REP_GATEWAY_TLS_KEY", ""), "TLS private key path")
	fs.IntVar(&cfg.HealthPort, "health-port", envOrDefaultInt("REP_GATEWAY_HEALTH_PORT", 0), "Separate health check port (0 = same as main)")
	sessionTTL := fs.String("session-key-ttl", envOrDefault("REP_GATEWAY_SESSION_KEY_TTL", defaultSessionTTL), "Session key TTL")
	fs.IntVar(&cfg.SessionKeyMaxRate, "session-key-max-rate", envOrDefaultInt("REP_GATEWAY_SESSION_KEY_MAX_RATE", defaultSessionMaxRate), "Session key max requests/min/IP")
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

// prescanManifestFlag scans args for --manifest or -manifest (flag or flag=value form)
// without going through the full flag.FlagSet (which would reject unknown flags).
func prescanManifestFlag(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		for _, prefix := range []string{"--manifest=", "-manifest="} {
			if strings.HasPrefix(arg, prefix) {
				return strings.TrimPrefix(arg, prefix)
			}
		}
		if (arg == "--manifest" || arg == "-manifest") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
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
