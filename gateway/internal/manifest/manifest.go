// Package manifest loads and parses .rep.yaml manifest files.
//
// The manifest declares expected environment variables, their security tiers,
// types, constraints, and gateway settings. Per REP-RFC-0001 §6, the manifest
// is a developer-facing contract checked at gateway startup.
//
// This package implements a hand-rolled parser for the REP manifest subset of
// YAML to preserve the zero external-dependency constraint. The supported
// syntax is documented below and matches the rep-manifest.schema.json schema.
//
// Supported YAML features:
//   - Top-level scalar key-value pairs (version: "0.1.0")
//   - Block mappings up to 3 levels deep (variables, settings)
//   - Scalar values: unquoted, single-quoted, double-quoted strings
//   - Boolean literals: true / false
//   - Integer literals
//   - Inline sequence literals: ["v1", "v2"]
//   - Block sequences with - prefix items
//   - Line comments: # ...
package manifest

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// VarDecl declares a single variable entry in the manifest.
type VarDecl struct {
	// Tier is the required security tier: "public", "sensitive", or "server".
	Tier string

	// Type is the value type for validation. Defaults to "string".
	// Valid: string | url | number | boolean | csv | json | enum
	Type string

	// Required means the variable must be present in the environment at startup.
	Required bool

	// Default holds the fallback value when Required is false and the variable
	// is absent. HasDefault distinguishes an explicit empty default from
	// "no default declared".
	Default    string
	HasDefault bool

	// Description is a human-readable description (informational only).
	Description string

	// Example is a sample value for documentation (informational only).
	Example string

	// Pattern is a Go-compatible regular expression the value must match.
	Pattern string

	// Values lists all allowed values for type: enum.
	Values []string

	// Deprecated marks the variable as deprecated; the gateway logs a warning
	// if it is present.
	Deprecated        bool
	DeprecatedMessage string
}

// Settings holds gateway configuration from the manifest settings block.
// These provide the lowest-priority defaults, overridden by REP_GATEWAY_*
// environment variables and CLI flags.
type Settings struct {
	StrictGuardrails      bool
	HotReload             bool
	HotReloadMode         string
	HotReloadPollInterval time.Duration
	SessionKeyTTL         time.Duration
	SessionKeyMaxRate     int
	AllowedOrigins        []string
}

// Manifest holds the fully parsed .rep.yaml contents.
type Manifest struct {
	// Version is the REP protocol version string (e.g. "0.1.0").
	Version string

	// Variables maps variable name (without REP_*_prefix) to its declaration.
	Variables map[string]*VarDecl

	// Settings holds optional gateway configuration from the manifest.
	// May be nil if the settings block is absent.
	Settings *Settings
}

// Load reads and parses a .rep.yaml manifest file at path.
// Returns a non-nil *Manifest on success. Returns an error if the file
// cannot be opened, read, or parsed.
func Load(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening manifest %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading manifest %q: %w", path, err)
	}

	m, err := parseManifest(lines)
	if err != nil {
		return nil, fmt.Errorf("parsing manifest %q: %w", path, err)
	}
	return m, nil
}

// Validate checks classified environment variables against the manifest
// declarations and returns an error listing all violations (missing required
// variables, type errors, pattern mismatches, bad enum values).
//
// public, sensitive, and server are name→value maps for the three tiers.
// Deprecated variables that are present cause a warning log entry; they do
// NOT count as errors.
func (m *Manifest) Validate(public, sensitive, server map[string]string, log func(msg string, args ...any)) error {
	if m == nil || len(m.Variables) == 0 {
		return nil
	}

	// Build a unified lookup: name → (value, exists).
	all := make(map[string]string, len(public)+len(sensitive)+len(server))
	for k, v := range public {
		all[k] = v
	}
	for k, v := range sensitive {
		all[k] = v
	}
	for k, v := range server {
		all[k] = v
	}

	var errs []string

	for name, decl := range m.Variables {
		value, exists := all[name]

		if !exists {
			if decl.Required {
				errs = append(errs, fmt.Sprintf("required variable %q is not set", name))
			}
			// Optional + absent: nothing to validate.
			continue
		}

		// Deprecated variable present → warning.
		if decl.Deprecated {
			msg := fmt.Sprintf("variable %q is deprecated", name)
			if decl.DeprecatedMessage != "" {
				msg += ": " + decl.DeprecatedMessage
			}
			if log != nil {
				log("rep.manifest.deprecated_var", "name", name, "message", msg)
			}
		}

		// Type validation.
		if err := validateType(name, value, decl); err != nil {
			errs = append(errs, err.Error())
			continue
		}

		// Pattern validation (applies to any type when declared).
		if decl.Pattern != "" {
			matched, err := regexp.MatchString(`^(?:`+decl.Pattern+`)$`, value)
			if err != nil {
				errs = append(errs, fmt.Sprintf("variable %q has invalid pattern expression %q: %v", name, decl.Pattern, err))
				continue
			}
			if !matched {
				errs = append(errs, fmt.Sprintf("variable %q value does not match pattern %q", name, decl.Pattern))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("manifest validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// validateType checks that value conforms to the declared type.
func validateType(name, value string, decl *VarDecl) error {
	switch decl.Type {
	case "url":
		u, err := url.ParseRequestURI(value)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("variable %q must be a valid URL, got %q", name, value)
		}
	case "number":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("variable %q must be a number, got %q", name, value)
		}
	case "boolean":
		lower := strings.ToLower(value)
		if lower != "true" && lower != "false" && lower != "1" && lower != "0" {
			return fmt.Errorf("variable %q must be a boolean (true/false/1/0), got %q", name, value)
		}
	case "enum":
		if len(decl.Values) > 0 {
			found := false
			for _, allowed := range decl.Values {
				if value == allowed {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("variable %q must be one of %v, got %q", name, decl.Values, value)
			}
		}
	case "string", "csv", "json", "":
		// No structural type validation; pattern covers string constraints.
	default:
		// Unknown type — log nothing, skip (forward compatibility).
	}
	return nil
}

// ---------------------------------------------------------------------------
// Hand-rolled YAML parser (REP manifest subset only)
// ---------------------------------------------------------------------------

type parserState int

const (
	stRoot        parserState = iota
	stVariables               // inside variables: block
	stVarProps                // inside a specific variable's property block
	stVarValues               // collecting multi-line `- item` for values:
	stSettings                // inside settings: block
	stSettOrigins             // collecting multi-line `- item` for allowed_origins:
)

func parseManifest(lines []string) (*Manifest, error) {
	m := &Manifest{
		Variables: make(map[string]*VarDecl),
	}

	state := stRoot
	var curVarName string
	var curVar *VarDecl

	for _, raw := range lines {
		// Strip inline comments — but only outside of quoted strings.
		raw = stripComment(raw)

		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		indent := countIndent(raw)

		// ── Root-level lines always reset state ──────────────────────────────
		if indent == 0 {
			state = stRoot
			key, val, _ := splitKV(trimmed)
			switch key {
			case "version":
				m.Version = unquoteYAML(val)
			case "variables":
				state = stVariables
			case "settings":
				if m.Settings == nil {
					m.Settings = defaultSettings()
				}
				state = stSettings
			}
			continue
		}

		// ── State-dependent handling ──────────────────────────────────────────
		switch state {

		case stVariables:
			// indent == 2 → new variable declaration
			if !strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, ":") {
				// Bare name with no colon — treat as variable name.
				curVarName = trimmed
				curVar = &VarDecl{Type: "string"}
				m.Variables[curVarName] = curVar
				state = stVarProps
				continue
			}
			name := strings.TrimSuffix(trimmed, ":")
			if !strings.Contains(name, ":") {
				// It's "VARNAME:" — a new variable block.
				curVarName = name
				curVar = &VarDecl{Type: "string"}
				m.Variables[curVarName] = curVar
				state = stVarProps
			}

		case stVarProps:
			if indent == 2 {
				// New variable at same level — store previous, start new.
				name := strings.TrimSuffix(trimmed, ":")
				curVarName = name
				curVar = &VarDecl{Type: "string"}
				m.Variables[curVarName] = curVar
				continue
			}
			if indent >= 4 {
				key, val, hasVal := splitKV(trimmed)
				switch key {
				case "tier":
					curVar.Tier = unquoteYAML(val)
				case "type":
					curVar.Type = unquoteYAML(val)
				case "required":
					curVar.Required = parseBoolLiteral(val)
				case "default":
					curVar.Default = unquoteYAML(val)
					curVar.HasDefault = true
				case "description":
					curVar.Description = unquoteYAML(val)
				case "example":
					curVar.Example = unquoteYAML(val)
				case "pattern":
					curVar.Pattern = unquoteYAML(val)
				case "deprecated":
					curVar.Deprecated = parseBoolLiteral(val)
				case "deprecated_message":
					curVar.DeprecatedMessage = unquoteYAML(val)
				case "values":
					if hasVal && strings.HasPrefix(strings.TrimSpace(val), "[") {
						curVar.Values = parseInlineSequence(val)
					} else if !hasVal {
						state = stVarValues
					}
				}
			}

		case stVarValues:
			// Collecting `- item` entries for values:.
			if strings.HasPrefix(trimmed, "- ") {
				curVar.Values = append(curVar.Values, unquoteYAML(strings.TrimPrefix(trimmed, "- ")))
				continue
			}
			// Anything else ends the list — fall through to stVarProps or stVariables.
			state = stVarProps
			if indent == 2 {
				name := strings.TrimSuffix(trimmed, ":")
				curVarName = name
				curVar = &VarDecl{Type: "string"}
				m.Variables[curVarName] = curVar
			} else if indent >= 4 {
				key, val, hasVal := splitKV(trimmed)
				applyVarProp(curVar, key, val, hasVal, func() { state = stVarValues })
			}

		case stSettings:
			if indent >= 2 && m.Settings != nil {
				key, val, hasVal := splitKV(trimmed)
				switch key {
				case "strict_guardrails":
					m.Settings.StrictGuardrails = parseBoolLiteral(val)
				case "hot_reload":
					m.Settings.HotReload = parseBoolLiteral(val)
				case "hot_reload_mode":
					m.Settings.HotReloadMode = unquoteYAML(val)
				case "hot_reload_poll_interval":
					if d, err := time.ParseDuration(unquoteYAML(val)); err == nil {
						m.Settings.HotReloadPollInterval = d
					}
				case "session_key_ttl":
					if d, err := time.ParseDuration(unquoteYAML(val)); err == nil {
						m.Settings.SessionKeyTTL = d
					}
				case "session_key_max_rate":
					if n, err := strconv.Atoi(val); err == nil {
						m.Settings.SessionKeyMaxRate = n
					}
				case "allowed_origins":
					if hasVal && strings.HasPrefix(strings.TrimSpace(val), "[") {
						m.Settings.AllowedOrigins = parseInlineSequence(val)
					} else if !hasVal {
						state = stSettOrigins
					}
				}
			}

		case stSettOrigins:
			if strings.HasPrefix(trimmed, "- ") {
				m.Settings.AllowedOrigins = append(m.Settings.AllowedOrigins, unquoteYAML(strings.TrimPrefix(trimmed, "- ")))
				continue
			}
			// End of list.
			state = stSettings
			if indent >= 2 && m.Settings != nil {
				key, val, hasVal := splitKV(trimmed)
				if key == "allowed_origins" {
					if hasVal && strings.HasPrefix(strings.TrimSpace(val), "[") {
						m.Settings.AllowedOrigins = parseInlineSequence(val)
					}
				} else {
					_ = key
					_ = val
					_ = hasVal
				}
			}
		}
	}

	return m, nil
}

// applyVarProp is a helper used when re-processing a line after ending a
// multi-line list.
func applyVarProp(v *VarDecl, key, val string, hasVal bool, startList func()) {
	switch key {
	case "tier":
		v.Tier = unquoteYAML(val)
	case "type":
		v.Type = unquoteYAML(val)
	case "required":
		v.Required = parseBoolLiteral(val)
	case "default":
		v.Default = unquoteYAML(val)
		v.HasDefault = true
	case "description":
		v.Description = unquoteYAML(val)
	case "example":
		v.Example = unquoteYAML(val)
	case "pattern":
		v.Pattern = unquoteYAML(val)
	case "deprecated":
		v.Deprecated = parseBoolLiteral(val)
	case "deprecated_message":
		v.DeprecatedMessage = unquoteYAML(val)
	case "values":
		if hasVal && strings.HasPrefix(strings.TrimSpace(val), "[") {
			v.Values = parseInlineSequence(val)
		} else if !hasVal {
			if startList != nil {
				startList()
			}
		}
	}
}

// defaultSettings returns a Settings struct populated with spec defaults.
func defaultSettings() *Settings {
	return &Settings{
		HotReloadMode:         "signal",
		HotReloadPollInterval: 30 * time.Second,
		SessionKeyTTL:         30 * time.Second,
		SessionKeyMaxRate:     10,
	}
}

// ---------------------------------------------------------------------------
// Parser helpers
// ---------------------------------------------------------------------------

// countIndent returns the number of leading spaces in s.
func countIndent(s string) int {
	n := 0
	for _, ch := range s {
		if ch == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}

// splitKV splits "key: value" into (key, value, hasValue).
// hasValue is false when there is no colon or the value portion is empty.
func splitKV(s string) (key, val string, hasVal bool) {
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return strings.TrimSpace(s), "", false
	}
	key = strings.TrimSpace(s[:idx])
	val = strings.TrimSpace(s[idx+1:])
	hasVal = val != ""
	return
}

// unquoteYAML removes surrounding single or double quotes from a YAML scalar.
func unquoteYAML(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			inner := s[1 : len(s)-1]
			// Unescape common double-quoted escape sequences.
			inner = strings.ReplaceAll(inner, `\"`, `"`)
			inner = strings.ReplaceAll(inner, `\\`, `\`)
			inner = strings.ReplaceAll(inner, `\n`, "\n")
			inner = strings.ReplaceAll(inner, `\t`, "\t")
			return inner
		}
	}
	return s
}

// parseBoolLiteral converts "true"/"false" (case-insensitive) to bool.
func parseBoolLiteral(s string) bool {
	return strings.EqualFold(strings.TrimSpace(s), "true")
}

// parseInlineSequence parses ["v1", "v2", "v3"] into a string slice.
// It handles quoted and unquoted items separated by commas.
func parseInlineSequence(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")

	var items []string
	for _, part := range strings.Split(s, ",") {
		item := unquoteYAML(strings.TrimSpace(part))
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

// stripComment removes a trailing YAML comment from a line, respecting quoted
// strings so that #-characters inside quotes are preserved.
func stripComment(line string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '#' && !inSingle && !inDouble:
			return line[:i]
		}
	}
	return line
}
