package extractor

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// AESKey is the shared key used by embed payload decryption (commonly used)
var AESKey = []byte{
	0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38,
	0x39, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36,
}

// DecryptPayload decrypts an AES-256-CTR encrypted embed payload
func DecryptPayload(encryptedData string) (string, error) {
	// Decode base64
	decoded, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	// Payload format: first 16 bytes = IV, rest = ciphertext
	if len(decoded) < 17 {
		return "", fmt.Errorf("encrypted data too short: %d bytes", len(decoded))
	}

	iv := decoded[:16]
	ciphertext := decoded[16:]

	// AES-CTR decryption
	block, err := aes.NewCipher(AESKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return string(plaintext), nil
}

// DecryptWithKey decrypts payload with a specific key
func DecryptWithKey(encryptedData string, key []byte) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	if len(decoded) < 17 {
		return "", fmt.Errorf("encrypted data too short: %d bytes", len(decoded))
	}

	iv := decoded[:16]
	ciphertext := decoded[16:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return string(plaintext), nil
}

// HexDecodeAndDecrypt decrypts hex-encoded payload
func HexDecodeAndDecrypt(encryptedData string) (string, error) {
	decoded, err := hex.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("hex decode failed: %w", err)
	}

	if len(decoded) < 17 {
		return "", fmt.Errorf("encrypted data too short: %d bytes", len(decoded))
	}

	iv := decoded[:16]
	ciphertext := decoded[16:]

	block, err := aes.NewCipher(AESKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return string(plaintext), nil
}
