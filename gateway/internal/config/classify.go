// Package config provides environment variable reading and classification.
//
// Per REP-RFC-0001 §3, variables are classified into three tiers:
//   - REP_PUBLIC_*   → PUBLIC tier  (plaintext in payload)
//   - REP_SENSITIVE_* → SENSITIVE tier (encrypted in payload)
//   - REP_SERVER_*   → SERVER tier  (never sent to client)
//
// Variables without the REP_ prefix are ignored entirely.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Tier represents the security classification of a variable.
type Tier int

const (
	TierPublic    Tier = iota // Plaintext, visible in page source.
	TierSensitive             // Encrypted, requires session key.
	TierServer                // Never leaves the gateway process.
)

// String returns the human-readable tier name.
func (t Tier) String() string {
	switch t {
	case TierPublic:
		return "public"
	case TierSensitive:
		return "sensitive"
	case TierServer:
		return "server"
	default:
		return "unknown"
	}
}

// Variable represents a classified environment variable.
type Variable struct {
	// Name is the variable name with the REP_<TIER>_ prefix stripped.
	// e.g., REP_PUBLIC_API_URL → "API_URL".
	Name string

	// Value is the raw string value.
	Value string

	// Tier is the security classification.
	Tier Tier

	// OriginalKey is the full environment variable name (for diagnostics).
	OriginalKey string
}

// ClassifiedVars holds variables grouped by tier.
type ClassifiedVars struct {
	Public    []Variable
	Sensitive []Variable
	Server    []Variable
}

// PublicMap returns a map of name → value for all PUBLIC tier variables.
func (cv *ClassifiedVars) PublicMap() map[string]string {
	m := make(map[string]string, len(cv.Public))
	for _, v := range cv.Public {
		m[v.Name] = v.Value
	}
	return m
}

// SensitiveMap returns a map of name → value for all SENSITIVE tier variables.
func (cv *ClassifiedVars) SensitiveMap() map[string]string {
	m := make(map[string]string, len(cv.Sensitive))
	for _, v := range cv.Sensitive {
		m[v.Name] = v.Value
	}
	return m
}

// ServerMap returns a map of name → value for all SERVER tier variables.
func (cv *ClassifiedVars) ServerMap() map[string]string {
	m := make(map[string]string, len(cv.Server))
	for _, v := range cv.Server {
		m[v.Name] = v.Value
	}
	return m
}

// ReadAndClassify reads environment variables, filters for the REP_ prefix,
// classifies them, strips prefixes, and validates uniqueness.
//
// When envFile is non-empty, the file is parsed first as a base layer.
// Process environment variables (os.Environ) are then overlaid on top,
// so real env vars always take precedence over the file.
//
// Per REP-RFC-0001 §3.2:
//   - Only REP_* prefixed variables are read.
//   - The classification prefix is stripped from the name.
//   - Names MUST be unique across all tiers after stripping.
func ReadAndClassify(envFile string) (*ClassifiedVars, error) {
	// Build a merged map: env file (base) + os.Environ() (override).
	merged := make(map[string]string)

	if envFile != "" {
		fileVars, err := ParseEnvFile(envFile)
		if err != nil {
			return nil, fmt.Errorf("reading env file: %w", err)
		}
		for k, v := range fileVars {
			merged[k] = v
		}
	}

	// Process environment overrides file values.
	for _, env := range os.Environ() {
		key, value, ok := strings.Cut(env, "=")
		if !ok {
			continue
		}
		merged[key] = value
	}

	// Classify the merged set.
	vars := &ClassifiedVars{}
	seen := make(map[string]string) // name → original key (for collision detection)

	for key, value := range merged {
		// Skip non-REP variables.
		if !strings.HasPrefix(key, "REP_") {
			continue
		}

		// Skip gateway configuration variables (REP_GATEWAY_*).
		if strings.HasPrefix(key, "REP_GATEWAY_") {
			continue
		}

		var v Variable
		v.OriginalKey = key
		v.Value = value

		switch {
		case strings.HasPrefix(key, "REP_PUBLIC_"):
			v.Name = strings.TrimPrefix(key, "REP_PUBLIC_")
			v.Tier = TierPublic
		case strings.HasPrefix(key, "REP_SENSITIVE_"):
			v.Name = strings.TrimPrefix(key, "REP_SENSITIVE_")
			v.Tier = TierSensitive
		case strings.HasPrefix(key, "REP_SERVER_"):
			v.Name = strings.TrimPrefix(key, "REP_SERVER_")
			v.Tier = TierServer
		default:
			continue
		}

		// Check for name collisions across tiers (§3.2 rule 4).
		if existing, exists := seen[v.Name]; exists {
			return nil, fmt.Errorf(
				"variable name collision: %q (from %s) conflicts with %q — names must be unique across tiers after prefix stripping",
				v.OriginalKey, v.Tier, existing,
			)
		}
		seen[v.Name] = v.OriginalKey

		// Classify into tier bucket.
		switch v.Tier {
		case TierPublic:
			vars.Public = append(vars.Public, v)
		case TierSensitive:
			vars.Sensitive = append(vars.Sensitive, v)
		case TierServer:
			vars.Server = append(vars.Server, v)
		}
	}

	return vars, nil
}
