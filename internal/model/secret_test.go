package model

import "testing"

// TestEncryptDecryptSecretRoundTrip verifies provider secrets are encrypted at rest and restored on demand.
// TestEncryptDecryptSecretRoundTrip 用于验证供应商密钥会先加密落地，再按需恢复明文。
func TestEncryptDecryptSecretRoundTrip(t *testing.T) {
	encrypted, err := EncryptSecret("sk-test-secret", "unit-test-key")
	if err != nil {
		t.Fatalf("EncryptSecret() error = %v", err)
	}
	if encrypted == "" || encrypted == "sk-test-secret" {
		t.Fatalf("expected encrypted payload, got %q", encrypted)
	}

	plain, err := DecryptSecret(encrypted, "unit-test-key")
	if err != nil {
		t.Fatalf("DecryptSecret() error = %v", err)
	}
	if plain != "sk-test-secret" {
		t.Fatalf("plain = %q, want %q", plain, "sk-test-secret")
	}
}

// TestMaskSecret verifies logs and transport never expose the full secret payload.
// TestMaskSecret 用于验证日志和传输层都不会暴露完整敏感密钥。
func TestMaskSecret(t *testing.T) {
	if got := MaskSecret("sk-123456789"); got != "sk-***89" {
		t.Fatalf("MaskSecret() = %q, want %q", got, "sk-***89")
	}
	if got := MaskSecret("short"); got != "***redacted***" {
		t.Fatalf("MaskSecret(short) = %q, want %q", got, "***redacted***")
	}
}
