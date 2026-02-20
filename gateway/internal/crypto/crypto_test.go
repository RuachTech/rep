package crypto

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerateKeys(t *testing.T) {
	keys, err := GenerateKeys()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(keys.EncryptionKey) != 32 {
		t.Errorf("expected 32-byte encryption key, got %d", len(keys.EncryptionKey))
	}
	if len(keys.HMACSecret) != 32 {
		t.Errorf("expected 32-byte HMAC key, got %d", len(keys.HMACSecret))
	}

	// Keys should not be all zeros.
	allZero := true
	for _, b := range keys.EncryptionKey {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("encryption key is all zeros")
	}
}

func TestGenerateKeys_Unique(t *testing.T) {
	k1, _ := GenerateKeys()
	k2, _ := GenerateKeys()

	if string(k1.EncryptionKey) == string(k2.EncryptionKey) {
		t.Error("two generated keys should not be identical")
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	keys, _ := GenerateKeys()
	input := map[string]string{
		"ANALYTICS_KEY": "UA-12345",
		"API_SECRET":    "s3cret-value",
	}
	integrity := "hmac-sha256:test-integrity-token"

	blob, err := EncryptSensitive(input, keys.EncryptionKey, integrity)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	if blob == "" {
		t.Fatal("expected non-empty blob")
	}

	plaintext, err := DecryptSensitive(blob, keys.EncryptionKey, integrity)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(plaintext, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	for k, v := range input {
		if result[k] != v {
			t.Errorf("key %q: expected %q, got %q", k, v, result[k])
		}
	}
}

func TestEncryptDecrypt_WrongKey(t *testing.T) {
	k1, _ := GenerateKeys()
	k2, _ := GenerateKeys()
	input := map[string]string{"KEY": "value"}
	integrity := "test"

	blob, err := EncryptSensitive(input, k1.EncryptionKey, integrity)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	_, err = DecryptSensitive(blob, k2.EncryptionKey, integrity)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestEncryptDecrypt_WrongAAD(t *testing.T) {
	keys, _ := GenerateKeys()
	input := map[string]string{"KEY": "value"}

	blob, err := EncryptSensitive(input, keys.EncryptionKey, "integrity-1")
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	_, err = DecryptSensitive(blob, keys.EncryptionKey, "integrity-2")
	if err == nil {
		t.Fatal("expected error when decrypting with wrong AAD")
	}
}

func TestEncryptSensitive_EmptyMap(t *testing.T) {
	keys, _ := GenerateKeys()

	blob, err := EncryptSensitive(map[string]string{}, keys.EncryptionKey, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blob != "" {
		t.Errorf("expected empty string for empty map, got %q", blob)
	}
}

func TestComputeIntegrity_Deterministic(t *testing.T) {
	keys, _ := GenerateKeys()
	publicMap := map[string]string{"A": "1", "B": "2"}

	h1 := ComputeIntegrity(publicMap, "blob", keys.HMACSecret)
	h2 := ComputeIntegrity(publicMap, "blob", keys.HMACSecret)

	if h1 != h2 {
		t.Errorf("integrity should be deterministic: %q != %q", h1, h2)
	}
}

func TestComputeIntegrity_Format(t *testing.T) {
	keys, _ := GenerateKeys()

	result := ComputeIntegrity(map[string]string{"K": "V"}, "", keys.HMACSecret)
	if !strings.HasPrefix(result, "hmac-sha256:") {
		t.Errorf("expected hmac-sha256: prefix, got %q", result)
	}

	// The part after the prefix should be valid base64.
	b64 := strings.TrimPrefix(result, "hmac-sha256:")
	if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
		t.Errorf("integrity suffix is not valid base64: %v", err)
	}
}

func TestComputeIntegrity_DifferentInputs(t *testing.T) {
	keys, _ := GenerateKeys()

	h1 := ComputeIntegrity(map[string]string{"A": "1"}, "", keys.HMACSecret)
	h2 := ComputeIntegrity(map[string]string{"A": "2"}, "", keys.HMACSecret)

	if h1 == h2 {
		t.Error("different inputs should produce different integrity tokens")
	}
}

func TestComputeSRI_Format(t *testing.T) {
	result := ComputeSRI([]byte(`{"public":{}}`))
	if !strings.HasPrefix(result, "sha256-") {
		t.Errorf("expected sha256- prefix, got %q", result)
	}

	b64 := strings.TrimPrefix(result, "sha256-")
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("SRI hash is not valid base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("expected 32-byte SHA-256 hash, got %d", len(decoded))
	}
}

func TestComputeSRI_Deterministic(t *testing.T) {
	content := []byte(`{"key":"value"}`)

	h1 := ComputeSRI(content)
	h2 := ComputeSRI(content)

	if h1 != h2 {
		t.Error("SRI hash should be deterministic")
	}
}

func TestCanonicalize_SortedKeys(t *testing.T) {
	m := map[string]string{"Z": "3", "A": "1", "M": "2"}
	result := canonicalize(m)

	expected := `{"A":"1","M":"2","Z":"3"}`
	if result != expected {
		t.Errorf("canonicalize: expected %s, got %s", expected, result)
	}
}

func TestCanonicalize_EmptyMap(t *testing.T) {
	result := canonicalize(map[string]string{})
	if result != "{}" {
		t.Errorf("expected {}, got %s", result)
	}
}

func TestDecryptSensitive_TooShort(t *testing.T) {
	// Less than 12 bytes (nonce size).
	shortBlob := base64.StdEncoding.EncodeToString([]byte("short"))
	keys, _ := GenerateKeys()

	_, err := DecryptSensitive(shortBlob, keys.EncryptionKey, "test")
	if err == nil {
		t.Fatal("expected error for too-short ciphertext")
	}
}

func TestDecryptSensitive_BadBase64(t *testing.T) {
	keys, _ := GenerateKeys()

	_, err := DecryptSensitive("not-valid-base64!!!", keys.EncryptionKey, "test")
	if err == nil {
		t.Fatal("expected error for bad base64")
	}
}

// ─── DeriveKey (HKDF-SHA256) tests ───────────────────────────────────────────

func TestDeriveKey_Deterministic(t *testing.T) {
	ikm := make([]byte, 32)
	salt := make([]byte, 32)
	for i := range ikm {
		ikm[i] = byte(i)
		salt[i] = byte(100 + i)
	}

	k1 := DeriveKey(ikm, salt, "rep-test", 32)
	k2 := DeriveKey(ikm, salt, "rep-test", 32)

	if string(k1) != string(k2) {
		t.Error("DeriveKey should be deterministic given identical inputs")
	}
}

func TestDeriveKey_DifferentSalts(t *testing.T) {
	ikm := make([]byte, 32)
	salt1 := make([]byte, 32)
	salt2 := make([]byte, 32)
	for i := range salt1 {
		salt1[i] = byte(i)
		salt2[i] = byte(200 - i)
	}

	k1 := DeriveKey(ikm, salt1, "rep-test", 32)
	k2 := DeriveKey(ikm, salt2, "rep-test", 32)

	if string(k1) == string(k2) {
		t.Error("different salts must produce different derived keys")
	}
}

func TestDeriveKey_DifferentInfo(t *testing.T) {
	ikm := make([]byte, 32)
	salt := make([]byte, 32)

	k1 := DeriveKey(ikm, salt, "rep-blob-encryption-v1", 32)
	k2 := DeriveKey(ikm, salt, "rep-hmac-v1", 32)

	if string(k1) == string(k2) {
		t.Error("different info strings must produce different derived keys")
	}
}

func TestDeriveKey_Length(t *testing.T) {
	ikm := make([]byte, 32)
	salt := make([]byte, 32)

	k := DeriveKey(ikm, salt, "rep-test", 32)
	if len(k) != 32 {
		t.Errorf("expected 32-byte output, got %d", len(k))
	}
}

func TestDeriveKey_IntegratesWithEncryptDecrypt(t *testing.T) {
	// Ensure a HKDF-derived key works end-to-end in AES-256-GCM.
	ikm := make([]byte, 32)
	salt := make([]byte, 32)
	for i := range ikm {
		ikm[i] = byte(i + 7)
		salt[i] = byte(i + 42)
	}

	derived := DeriveKey(ikm, salt, "rep-blob-encryption-v1", 32)
	input := map[string]string{"SECRET": "from-hkdf-key"}
	integrity := "hmac-sha256:test-aad"

	blob, err := EncryptSensitive(input, derived, integrity)
	if err != nil {
		t.Fatalf("encrypt with derived key: %v", err)
	}

	plain, err := DecryptSensitive(blob, derived, integrity)
	if err != nil {
		t.Fatalf("decrypt with derived key: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(plain, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["SECRET"] != "from-hkdf-key" {
		t.Errorf("expected from-hkdf-key, got %s", result["SECRET"])
	}
}
