package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"

	"github.com/emillamm/envx"
)

// Sentinel errors for encryption/decryption failures
var (
	ErrMissingKey        = errors.New("missing encryption key in environment")
	ErrInvalidKey        = errors.New("invalid encryption key: must be 32 bytes (256-bit)")
	ErrInvalidCiphertext = errors.New("ciphertext too short or invalid")
	ErrDecryptionFailed  = errors.New("decryption failed")
)

// EncryptSymmetric encrypts plaintext data using AES-256-GCM
// The encryption key is loaded from the SECURE_KEY environment variable (base64 encoded)
// Returns ciphertext with prepended nonce
func EncryptSymmetric(env envx.EnvX, plaintext []byte) ([]byte, error) {
	// Get encryption key from environment
	keyB64 := env("SECURE_KEY")
	if keyB64 == "" {
		return nil, ErrMissingKey
	}

	// Decode the base64-encoded key
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, errors.New("failed to decode encryption key: " + err.Error())
	}

	// Verify key length (must be 32 bytes for AES-256)
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("failed to create cipher: " + err.Error())
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("failed to create GCM: " + err.Error())
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.New("failed to generate nonce: " + err.Error())
	}

	// Encrypt the plaintext
	// Seal appends the ciphertext and authentication tag to nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// DecryptSymmetric decrypts ciphertext data using AES-256-GCM
// The encryption key is loaded from the SECURE_KEY environment variable (base64 encoded)
// Expects ciphertext with prepended nonce
func DecryptSymmetric(env envx.EnvX, ciphertext []byte) ([]byte, error) {
	// Get encryption key from environment
	keyB64 := env("SECURE_KEY")
	if keyB64 == "" {
		return nil, ErrMissingKey
	}

	// Decode the base64-encoded key
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, errors.New("failed to decode encryption key: " + err.Error())
	}

	// Verify key length (must be 32 bytes for AES-256)
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("failed to create cipher: " + err.Error())
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("failed to create GCM: " + err.Error())
	}

	// Verify ciphertext length
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt the ciphertext
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptSymmetricString encrypts a string and returns base64-encoded ciphertext
func EncryptSymmetricString(env envx.EnvX, plaintext string) (string, error) {
	ciphertext, err := EncryptSymmetric(env, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSymmetricString decrypts a base64-encoded ciphertext string and returns the plaintext
func DecryptSymmetricString(env envx.EnvX, ciphertextB64 string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", errors.New("failed to decode ciphertext: " + err.Error())
	}

	plaintext, err := DecryptSymmetric(env, ciphertext)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
