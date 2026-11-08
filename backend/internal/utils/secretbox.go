package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

func DecodeMasterKey(raw string) ([]byte, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return nil, errors.New("MASTER_ENCRYPTION_KEY is required")
	}

	// Try base64 first (common for 32-byte secrets).
	if decoded, err := base64.StdEncoding.DecodeString(normalized); err == nil {
		if len(decoded) != 32 {
			return nil, errors.New("MASTER_ENCRYPTION_KEY must decode to 32 bytes")
		}
		return decoded, nil
	}

	// Fallback: hex.
	decoded, err := hex.DecodeString(normalized)
	if err != nil {
		return nil, errors.New("MASTER_ENCRYPTION_KEY must be base64 or hex")
	}
	if len(decoded) != 32 {
		return nil, errors.New("MASTER_ENCRYPTION_KEY must decode to 32 bytes")
	}
	return decoded, nil
}

func EncryptInitialPassword(key []byte, plaintext string) (ciphertextB64 string, nonceB64 string, err error) {
	if len(key) != 32 {
		return "", "", errors.New("encryption key must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(nonce), nil
}

func DecryptInitialPassword(key []byte, ciphertextB64 string, nonceB64 string) (string, error) {
	if len(key) != 32 {
		return "", errors.New("encryption key must be 32 bytes")
	}

	nonce, err := base64.StdEncoding.DecodeString(strings.TrimSpace(nonceB64))
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ciphertextB64))
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(nonce) != gcm.NonceSize() {
		return "", errors.New("invalid nonce size")
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
