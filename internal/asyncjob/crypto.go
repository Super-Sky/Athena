// crypto.go encrypts asynchronous job payloads and results before durable persistence.
// crypto.go 在异步 job payload 和 result 写入持久化前执行加密。
package asyncjob

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

const minimumEncryptionKeyBytes = 32

// Codec encrypts and decrypts async job request/result payloads with authenticated data.
// Codec 使用附加认证数据加解密 async job 的请求与结果 payload。
type Codec struct {
	aead cipher.AEAD
}

// NewCodec derives the AES-GCM key from the configured secret and validates minimum entropy length.
// NewCodec 从配置 secret 派生 AES-GCM key，并校验最小熵长度。
func NewCodec(secret string) (*Codec, error) {
	secret = strings.TrimSpace(secret)
	if len([]byte(secret)) < minimumEncryptionKeyBytes {
		return nil, fmt.Errorf("async job encryption key must be at least %d bytes", minimumEncryptionKeyBytes)
	}
	digest := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(digest[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Codec{aead: aead}, nil
}

// Encrypt seals one payload with the job-specific associated data.
// Encrypt 使用 job 相关的附加认证数据密封单个 payload。
func (c *Codec) Encrypt(plain []byte, associatedData string) (string, error) {
	if c == nil || c.aead == nil {
		return "", fmt.Errorf("async job codec is not configured")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, plain, []byte(associatedData))
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Decrypt opens one payload and rejects mismatched associated data.
// Decrypt 打开单个 payload，并拒绝附加认证数据不匹配的内容。
func (c *Codec) Decrypt(encoded string, associatedData string) ([]byte, error) {
	if c == nil || c.aead == nil {
		return nil, fmt.Errorf("async job codec is not configured")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode async job payload: %w", err)
	}
	if len(payload) < c.aead.NonceSize() {
		return nil, fmt.Errorf("async job payload is too short")
	}
	nonce, ciphertext := payload[:c.aead.NonceSize()], payload[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, ciphertext, []byte(associatedData))
	if err != nil {
		return nil, fmt.Errorf("decrypt async job payload: %w", err)
	}
	return plain, nil
}
