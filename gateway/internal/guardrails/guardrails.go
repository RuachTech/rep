// Package guardrails implements automatic secret detection for PUBLIC tier
// variables, as specified in REP-RFC-0001 §3.3.
//
// The guardrails scan REP_PUBLIC_* values for patterns that indicate they may
// be misclassified secrets: high Shannon entropy, known key formats, and
// length anomalies.
package guardrails

import (
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/ruachtech/rep/gateway/internal/config"
	"github.com/ruachtech/rep/gateway/internal/manifest"
)

// Warning represents a guardrail detection event.
type Warning struct {
	VariableName  string // Name without prefix (e.g., "API_KEY").
	OriginalKey   string // Full env var name (e.g., "REP_PUBLIC_API_KEY").
	DetectionType string // "high_entropy", "known_format", "length_anomaly".
	Message       string // Human-readable explanation.
}

// Result contains the outcome of a guardrail scan.
type Result struct {
	Warnings []Warning
}

// HasWarnings returns true if any warnings were detected.
func (r *Result) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// knownSecretPrefixes maps known API key/token prefixes to their service names.
var knownSecretPrefixes = []struct {
	prefix  string
	service string
}{
	{"AKIA", "AWS Access Key"},
	{"ASIA", "AWS Temporary Access Key"},
	{"eyJ", "JWT Token"},
	{"ghp_", "GitHub Personal Access Token"},
	{"gho_", "GitHub OAuth Token"},
	{"ghs_", "GitHub Server Token"},
	{"ghr_", "GitHub Refresh Token"},
	{"github_pat_", "GitHub Fine-Grained PAT"},
	{"sk_live_", "Stripe Secret Key"},
	{"rk_live_", "Stripe Restricted Key"},
	{"sk-", "OpenAI API Key"},
	{"xoxb-", "Slack Bot Token"},
	{"xoxp-", "Slack User Token"},
	{"xoxs-", "Slack App Token"},
	{"SG.", "SendGrid API Key"},
	{"-----BEGIN", "Private Key / Certificate"},
	{"AGE-SECRET-KEY-", "age Encryption Key"},
}

// Scan checks all PUBLIC tier variables for potential misclassification.
//
// Per REP-RFC-0001 §3.3, the gateway MUST scan and MUST log warnings.
// If strict mode is enabled, the caller should treat warnings as errors.
//
// m is the loaded manifest, or nil. A variable it declares as type csv is
// scanned element by element (split on commas, trimmed), with every
// heuristic applied to every element (§3.3).
func Scan(vars *config.ClassifiedVars, m *manifest.Manifest, logger *slog.Logger) *Result {
	result := &Result{}

	for _, v := range vars.Public {
		if !isCSV(m, v.Name) {
			result.scanValue(v, v.Value, 0, logger)
			continue
		}
		for i, elem := range strings.Split(v.Value, ",") {
			result.scanValue(v, strings.TrimSpace(elem), i+1, logger)
		}
	}

	return result
}

// isCSV reports whether the manifest declares name with type csv.
func isCSV(m *manifest.Manifest, name string) bool {
	if m == nil {
		return false
	}
	d := m.Variables[name]
	return d != nil && d.Type == "csv"
}

// scanValue runs every heuristic on value, which is the whole of v's value
// when element is 0 and its element'th csv element (1-based) otherwise.
func (r *Result) scanValue(v config.Variable, value string, element int, logger *slog.Logger) {
	warn := func(detectionType, message string, extra ...any) {
		args := append([]any{"variable_name", v.Name, "detection_type", detectionType}, extra...)
		if element > 0 {
			message = fmt.Sprintf("csv element %d: %s", element, message)
			args = append(args, "csv_element", element)
		}
		r.Warnings = append(r.Warnings, Warning{
			VariableName:  v.Name,
			OriginalKey:   v.OriginalKey,
			DetectionType: detectionType,
			Message:       message,
		})
		logger.Warn("rep.guardrail.warning", args...)
	}

	// Check known secret formats.
	for _, kp := range knownSecretPrefixes {
		if strings.HasPrefix(value, kp.prefix) {
			msg := fmt.Sprintf("value matches known %s format (prefix: %s)", kp.service, kp.prefix)
			warn("known_format", msg, "detail", msg)
			break // One match is enough per value.
		}
	}

	// Check Shannon entropy.
	if entropy := shannonEntropy(value); entropy > 4.5 && len(value) > 16 {
		warn("high_entropy",
			fmt.Sprintf("value has high entropy (%.2f bits/char) — may be a secret", entropy),
			"entropy", fmt.Sprintf("%.2f", entropy))
	}

	// Check length anomaly.
	if len(value) > 64 && !strings.Contains(value, " ") && !strings.HasPrefix(value, "http") {
		warn("length_anomaly",
			fmt.Sprintf("value is %d chars with no spaces and no URL prefix — may be an encoded secret", len(value)),
			"length", len(value))
	}
}

// shannonEntropy calculates the Shannon entropy (bits per character) of a string.
// High entropy (>4.5) typically indicates random/secret-like content.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	freq := make(map[rune]int)
	for _, c := range s {
		freq[c]++
	}

	length := float64(len([]rune(s)))
	entropy := 0.0
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}
