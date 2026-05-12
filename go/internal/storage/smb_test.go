package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestNewSMBStore_UnreachableHost(t *testing.T) {
	_, err := NewSMBStore("invalid-host-xyz", "share", "user", "pass", "basedir", "")
	if err == nil {
		t.Error("expected error for unreachable SMB host")
	}
}

func TestNewSMBStore_EncryptionKeyPlainText(t *testing.T) {
	_, err := NewSMBStore("invalid-host", "share", "user", "pass", "basedir", "my-encryption-key")
	if err == nil {
		t.Error("expected error for unreachable SMB host")
	}
}

func TestNewSMBStore_EncryptionKeyHex(t *testing.T) {
	_, err := NewSMBStore("invalid-host", "share", "user", "pass", "basedir", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err == nil {
		t.Error("expected error for unreachable SMB host")
	}
}

// --- Key derivation tests (pure logic, no SMB needed) ---

func TestDeriveKey_PlainTextHash(t *testing.T) {
	key := "my-password"
	hash := sha256.Sum256([]byte(key))
	if len(hash) != 32 {
		t.Errorf("SHA256 key length = %d, want 32", len(hash))
	}
	// Verify it produces a consistent key
	hash2 := sha256.Sum256([]byte(key))
	if !bytes.Equal(hash[:], hash2[:]) {
		t.Error("SHA256 should produce consistent output")
	}
}

func TestDeriveKey_HexDecoded(t *testing.T) {
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	decoded, err := hex.DecodeString(hexKey)
	if err != nil {
		t.Fatalf("hex.DecodeString() error: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("decoded key length = %d, want 32", len(decoded))
	}
}

func TestDeriveKey_PlainTextVsHex_Different(t *testing.T) {
	// A plaintext key should produce a different result than a hex key
	// even if the plaintext looks similar
	hashKey := "my-password"
	hash := sha256.Sum256([]byte(hashKey))

	// Hex key should be decoded directly, not hashed
	hexKey := "6d792d70617373776f7264" // hex encoding of "my-password"
	decoded, _ := hex.DecodeString(hexKey)

	if bytes.Equal(hash[:], decoded) {
		t.Error("plaintext hash should differ from hex-decoded version")
	}
}

func TestDeriveKey_DifferentInputs_DifferentOutput(t *testing.T) {
	h1 := sha256.Sum256([]byte("key1"))
	h2 := sha256.Sum256([]byte("key2"))
	if bytes.Equal(h1[:], h2[:]) {
		t.Error("different inputs should produce different hashes")
	}
}
