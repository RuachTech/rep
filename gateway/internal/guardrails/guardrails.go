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

	"github.com/ruach-tech/rep/gateway/internal/config"
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
func Scan(vars *config.ClassifiedVars, logger *slog.Logger) *Result {
	result := &Result{}

	for _, v := range vars.Public {
		// Check known secret formats.
		for _, kp := range knownSecretPrefixes {
			if strings.HasPrefix(v.Value, kp.prefix) {
				w := Warning{
					VariableName:  v.Name,
					OriginalKey:   v.OriginalKey,
					DetectionType: "known_format",
					Message:       fmt.Sprintf("value matches known %s format (prefix: %s)", kp.service, kp.prefix),
				}
				result.Warnings = append(result.Warnings, w)
				logger.Warn("rep.guardrail.warning",
					"variable_name", v.Name,
					"detection_type", "known_format",
					"detail", w.Message,
				)
				break // One match is enough per variable.
			}
		}

		// Check Shannon entropy.
		entropy := shannonEntropy(v.Value)
		if entropy > 4.5 && len(v.Value) > 16 {
			w := Warning{
				VariableName:  v.Name,
				OriginalKey:   v.OriginalKey,
				DetectionType: "high_entropy",
				Message:       fmt.Sprintf("value has high entropy (%.2f bits/char) — may be a secret", entropy),
			}
			result.Warnings = append(result.Warnings, w)
			logger.Warn("rep.guardrail.warning",
				"variable_name", v.Name,
				"detection_type", "high_entropy",
				"entropy", fmt.Sprintf("%.2f", entropy),
			)
		}

		// Check length anomaly.
		if len(v.Value) > 64 && !strings.Contains(v.Value, " ") && !strings.HasPrefix(v.Value, "http") {
			w := Warning{
				VariableName:  v.Name,
				OriginalKey:   v.OriginalKey,
				DetectionType: "length_anomaly",
				Message:       fmt.Sprintf("value is %d chars with no spaces and no URL prefix — may be an encoded secret", len(v.Value)),
			}
			result.Warnings = append(result.Warnings, w)
			logger.Warn("rep.guardrail.warning",
				"variable_name", v.Name,
				"detection_type", "length_anomaly",
				"length", len(v.Value),
			)
		}
	}

	return result
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
