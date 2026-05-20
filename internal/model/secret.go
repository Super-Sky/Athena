// secret.go handles encryption and decryption of persisted model provider secrets.
// secret.go 负责已持久化模型供应商密钥的加解密处理。
package model

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// EncryptSecret protects one sensitive provider API key before it is persisted.
// EncryptSecret 会在供应商 API key 落库前对其进行加密保护。
func EncryptSecret(secret string, encryptionKey string) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", nil
	}
	block, err := aes.NewCipher(hashEncryptionKey(encryptionKey))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(secret), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret restores one encrypted provider API key for runtime use.
// DecryptSecret 会在运行时把加密后的供应商 API key 恢复为明文。
func DecryptSecret(payload string, encryptionKey string) (string, error) {
	if strings.TrimSpace(payload) == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("decode encrypted secret failed: %w", err)
	}
	block, err := aes.NewCipher(hashEncryptionKey(encryptionKey))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted secret payload is too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt encrypted secret failed: %w", err)
	}
	return string(plain), nil
}

// MaskSecret returns one transport-safe and log-safe summary for a provider secret.
// MaskSecret 会返回适合 transport 与日志打印的供应商敏感信息摘要。
func MaskSecret(secret string) string {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 6 {
		return "***redacted***"
	}
	return fmt.Sprintf("%s***%s", trimmed[:3], trimmed[len(trimmed)-2:])
}

func hashEncryptionKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
