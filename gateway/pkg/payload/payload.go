// Package payload constructs and serialises the REP JSON payload
// that is injected into HTML documents.
//
// See REP-RFC-0001 §4.3 (HTML injection) and §8.1 (wire format).
package payload

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ruach-tech/rep/gateway/internal/config"
	repcrypto "github.com/ruach-tech/rep/gateway/internal/crypto"
)

// Payload is the JSON structure injected into HTML documents.
// See REP-RFC-0001 §8.1 for the schema.
type Payload struct {
	Public    map[string]string `json:"public"`
	Sensitive string            `json:"sensitive,omitempty"`
	Meta      Meta              `json:"_meta"`
}

// Meta contains metadata about the payload.
type Meta struct {
	Version    string `json:"version"`
	InjectedAt string `json:"injected_at"`
	Integrity  string `json:"integrity"`
	KeyEndpoint string `json:"key_endpoint,omitempty"`
	HotReload  string `json:"hot_reload,omitempty"`
	TTL        int    `json:"ttl"`
}

// Builder constructs REP payloads from classified variables.
type Builder struct {
	keys       *repcrypto.Keys
	version    string
	hotReload  bool
}

// NewBuilder creates a payload builder with the given cryptographic keys.
func NewBuilder(keys *repcrypto.Keys, version string, hotReload bool) *Builder {
	return &Builder{
		keys:      keys,
		version:   version,
		hotReload: hotReload,
	}
}

// Build constructs the full REP payload from classified variables.
//
// This performs the following steps per §4.2 (startup sequence steps 7–9):
//  1. Computes HMAC integrity token over public + sensitive data.
//  2. Encrypts sensitive variables using AES-256-GCM.
//  3. Constructs the JSON payload object.
func (b *Builder) Build(vars *config.ClassifiedVars) (*Payload, error) {
	publicMap := vars.PublicMap()
	sensitiveMap := vars.SensitiveMap()

	// Step 1: Compute the integrity token over the public vars only.
	// This value is stored in _meta.integrity AND used as AAD for AES-GCM
	// encryption, so the SDK can use _payload._meta.integrity as AAD for
	// decryption (per §8.2). A single-pass approach avoids the circular
	// dependency that arises when trying to include the encrypted blob in
	// its own encryption AAD.
	integrity := repcrypto.ComputeIntegrity(publicMap, "", b.keys.HMACSecret)

	// Step 2: Encrypt sensitive variables, binding them to integrity via AAD.
	var sensitiveBlob string
	if len(sensitiveMap) > 0 {
		var err error
		sensitiveBlob, err = repcrypto.EncryptSensitive(sensitiveMap, b.keys.EncryptionKey, integrity)
		if err != nil {
			return nil, fmt.Errorf("encrypting sensitive vars: %w", err)
		}
	}

	// Construct the payload.
	p := &Payload{
		Public:    publicMap,
		Sensitive: sensitiveBlob,
		Meta: Meta{
			Version:    b.version,
			InjectedAt: time.Now().UTC().Format(time.RFC3339Nano),
			Integrity:  integrity,
			TTL:        0,
		},
	}

	// Add session key endpoint if sensitive vars exist.
	if len(sensitiveMap) > 0 {
		p.Meta.KeyEndpoint = "/rep/session-key"
	}

	// Add hot reload endpoint if enabled.
	if b.hotReload {
		p.Meta.HotReload = "/rep/changes"
	}

	return p, nil
}

// MarshalJSON serialises the payload to JSON bytes.
func (p *Payload) MarshalJSON() ([]byte, error) {
	// Use an alias to avoid infinite recursion.
	type PayloadAlias Payload
	return json.Marshal((*PayloadAlias)(p))
}

// ToJSON serialises the payload and returns the bytes.
func (p *Payload) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// ScriptTag generates the full <script> block for HTML injection.
//
// Per §4.3, the block has:
//   - id="__rep__" for SDK discovery
//   - type="application/json" to prevent execution
//   - data-rep-version for protocol version
//   - data-rep-integrity for SRI verification
func (p *Payload) ScriptTag() (string, error) {
	jsonBytes, err := p.ToJSON()
	if err != nil {
		return "", fmt.Errorf("serialising payload: %w", err)
	}

	sri := repcrypto.ComputeSRI(jsonBytes)

	return fmt.Sprintf(
		`<script id="__rep__" type="application/json" data-rep-version="%s" data-rep-integrity="%s">%s</script>`,
		p.Meta.Version,
		sri,
		string(jsonBytes),
	), nil
}
