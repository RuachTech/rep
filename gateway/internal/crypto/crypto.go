// Package crypto handles encryption of SENSITIVE tier variables and HMAC
// integrity computation for the REP payload.
//
// Per REP-RFC-0001 §8.2:
//   - Algorithm: AES-256-GCM
//   - Key: Ephemeral, generated at gateway startup
//   - Nonce: 12-byte random per encryption
//   - Blob format: [nonce (12B)][ciphertext][auth tag (16B)]
//
// Per REP-RFC-0001 §8.3:
//   - Integrity: HMAC-SHA256 over canonicalize(public) + "|" + sensitive
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
)

// Keys holds the ephemeral cryptographic material generated at gateway startup.
type Keys struct {
	// EncryptionKey is the AES-256 key for SENSITIVE tier encryption (32 bytes).
	EncryptionKey []byte

	// HMACSecret is the HMAC-SHA256 key for payload integrity (32 bytes).
	HMACSecret []byte
}

// GenerateKeys creates fresh ephemeral keys.
// These exist only in memory and are never persisted or transmitted.
func GenerateKeys() (*Keys, error) {
	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		return nil, fmt.Errorf("generating encryption key: %w", err)
	}

	hmacKey := make([]byte, 32)
	if _, err := rand.Read(hmacKey); err != nil {
		return nil, fmt.Errorf("generating HMAC key: %w", err)
	}

	return &Keys{
		EncryptionKey: encKey,
		HMACSecret:    hmacKey,
	}, nil
}

// EncryptSensitive encrypts the sensitive variables map using AES-256-GCM.
// Returns a base64-encoded blob: [nonce (12B)][ciphertext][auth tag (16B)].
//
// The integrityToken is used as additional authenticated data (AAD), binding
// the encrypted blob to the integrity token per §8.2.
func EncryptSensitive(sensitiveMap map[string]string, key []byte, integrityToken string) (string, error) {
	if len(sensitiveMap) == 0 {
		return "", nil
	}

	// Marshal the sensitive map to JSON.
	plaintext, err := json.Marshal(sensitiveMap)
	if err != nil {
		return "", fmt.Errorf("marshalling sensitive vars: %w", err)
	}

	// Create AES cipher.
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating AES cipher: %w", err)
	}

	// Create GCM.
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	// Generate random nonce.
	nonce := make([]byte, gcm.NonceSize()) // 12 bytes for GCM.
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	// Encrypt with AAD.
	aad := []byte(integrityToken)
	ciphertext := gcm.Seal(nonce, nonce, plaintext, aad) // Prepends nonce to output.

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSensitive decrypts a base64-encoded AES-256-GCM blob.
// Returns the plaintext JSON bytes of the sensitive variables map.
func DecryptSensitive(blob string, key []byte, integrityToken string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	aad := []byte(integrityToken)

	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// ComputeIntegrity computes the HMAC-SHA256 integrity token over the payload.
//
// Per §8.3: message = canonicalize(public) + "|" + sensitive
// Returns the formatted string "hmac-sha256:{base64_signature}".
func ComputeIntegrity(publicMap map[string]string, sensitiveBlob string, hmacKey []byte) string {
	canonical := canonicalize(publicMap)
	message := canonical + "|" + sensitiveBlob

	mac := hmac.New(sha256.New, hmacKey)
	mac.Write([]byte(message))
	sig := mac.Sum(nil)

	return "hmac-sha256:" + base64.StdEncoding.EncodeToString(sig)
}

// ComputeSRI computes the SHA-256 Subresource Integrity hash of the JSON payload.
// Returns the formatted string "sha256-{base64_hash}" for the data-rep-integrity attribute.
func ComputeSRI(jsonContent []byte) string {
	hash := sha256.Sum256(jsonContent)
	return "sha256-" + base64.StdEncoding.EncodeToString(hash[:])
}

// canonicalize produces a deterministic JSON string from a map (sorted keys, no extra whitespace).
func canonicalize(m map[string]string) string {
	// Sort keys.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build deterministic JSON.
	b, _ := json.Marshal(orderedMap{keys: keys, m: m})
	return string(b)
}

// orderedMap is a helper to produce JSON with sorted keys.
type orderedMap struct {
	keys []string
	m    map[string]string
}

// MarshalJSON implements json.Marshaler with sorted keys.
func (o orderedMap) MarshalJSON() ([]byte, error) {
	buf := []byte("{")
	for i, k := range o.keys {
		if i > 0 {
			buf = append(buf, ',')
		}
		key, _ := json.Marshal(k)
		val, _ := json.Marshal(o.m[k])
		buf = append(buf, key...)
		buf = append(buf, ':')
		buf = append(buf, val...)
	}
	buf = append(buf, '}')
	return buf, nil
}
