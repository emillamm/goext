package secure

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
)

// mockEnv creates a mock environment function for testing
func mockEnv(key string) func(string) string {
	return func(envKey string) string {
		if envKey == "AES_GCM_KEY" {
			return key
		}
		return ""
	}
}

// generateTestKey generates a random 32-byte AES-256 key
func generateTestKey() string {
	key := make([]byte, 32)
	rand.Read(key)
	return base64.StdEncoding.EncodeToString(key)
}

func TestEncryptDecryptSymmetric(t *testing.T) {
	key := generateTestKey()
	env := mockEnv(key)

	testCases := []struct {
		name      string
		plaintext []byte
	}{
		{"standard text", []byte("Hello, World!")},
		{"empty data", []byte{}},
		{"binary data", []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}},
		{"unicode text", []byte("Hello 世界 🌍")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := EncryptSymmetric(env, tc.plaintext)
			if err != nil {
				t.Fatalf("EncryptSymmetric failed: %v", err)
			}

			decrypted, err := DecryptSymmetric(env, ciphertext)
			if err != nil {
				t.Fatalf("DecryptSymmetric failed: %v", err)
			}

			if string(decrypted) != string(tc.plaintext) {
				t.Errorf("plaintext mismatch: expected %v, got %v", tc.plaintext, decrypted)
			}
		})
	}

	t.Run("different ciphertexts for same plaintext", func(t *testing.T) {
		plaintext := []byte("secret")
		ciphertext1, _ := EncryptSymmetric(env, plaintext)
		ciphertext2, _ := EncryptSymmetric(env, plaintext)

		if string(ciphertext1) == string(ciphertext2) {
			t.Fatal("expected different ciphertexts due to random nonce")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		emptyEnv := mockEnv("")
		_, err := EncryptSymmetric(emptyEnv, []byte("test"))
		if !errors.Is(err, ErrMissingKey) {
			t.Fatalf("expected ErrMissingKey, got: %v", err)
		}
	})

	t.Run("invalid key length", func(t *testing.T) {
		shortKey := make([]byte, 16)
		rand.Read(shortKey)
		invalidEnv := mockEnv(base64.StdEncoding.EncodeToString(shortKey))

		_, err := EncryptSymmetric(invalidEnv, []byte("test"))
		if !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("expected ErrInvalidKey, got: %v", err)
		}
	})

	t.Run("decryption with wrong key", func(t *testing.T) {
		plaintext := []byte("secret")
		ciphertext, _ := EncryptSymmetric(env, plaintext)

		differentEnv := mockEnv(generateTestKey())
		_, err := DecryptSymmetric(differentEnv, ciphertext)
		if !errors.Is(err, ErrDecryptionFailed) {
			t.Fatalf("expected ErrDecryptionFailed, got: %v", err)
		}
	})

	t.Run("corrupted ciphertext", func(t *testing.T) {
		plaintext := []byte("secret")
		ciphertext, _ := EncryptSymmetric(env, plaintext)
		ciphertext[len(ciphertext)-1] ^= 0xFF

		_, err := DecryptSymmetric(env, ciphertext)
		if !errors.Is(err, ErrDecryptionFailed) {
			t.Fatalf("expected ErrDecryptionFailed, got: %v", err)
		}
	})
}

func TestEncryptDecryptSymmetricString(t *testing.T) {
	key := generateTestKey()
	env := mockEnv(key)

	testCases := []struct {
		name      string
		plaintext string
	}{
		{"standard text", "Hello, World!"},
		{"empty string", ""},
		{"unicode text", "Hello 世界 🌍"},
		{"special characters", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := EncryptSymmetricString(env, tc.plaintext)
			if err != nil {
				t.Fatalf("EncryptSymmetricString failed: %v", err)
			}

			// Verify it's valid base64
			if _, err := base64.StdEncoding.DecodeString(ciphertext); err != nil {
				t.Fatalf("ciphertext is not valid base64: %v", err)
			}

			decrypted, err := DecryptSymmetricString(env, ciphertext)
			if err != nil {
				t.Fatalf("DecryptSymmetricString failed: %v", err)
			}

			if decrypted != tc.plaintext {
				t.Errorf("plaintext mismatch: expected %s, got %s", tc.plaintext, decrypted)
			}
		})
	}

	t.Run("invalid base64 ciphertext", func(t *testing.T) {
		_, err := DecryptSymmetricString(env, "not-valid-base64!")
		if err == nil {
			t.Fatal("expected error for invalid base64")
		}
	})
}
