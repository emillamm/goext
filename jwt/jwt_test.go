package jwt

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// mockEnv creates a mock environment function for testing
func mockEnv(privateKey, publicKey string) func(string) string {
	return func(key string) string {
		switch key {
		case "JWT_PRIVATE_KEY":
			return privateKey
		case "JWT_PUBLIC_KEY":
			return publicKey
		default:
			return ""
		}
	}
}

// generateTestKeys generates a test Ed25519 key pair
func generateTestKeys() (privateKeyB64, publicKeyB64 string) {
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	privateKeyB64 = base64.StdEncoding.EncodeToString(privateKey)
	publicKeyB64 = base64.StdEncoding.EncodeToString(publicKey)
	return
}

func TestBuildToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	env := mockEnv(privateKey, publicKey)

	userID := uuid.New()
	deviceID := "test-device-123"

	t.Run("successful token creation", func(t *testing.T) {
		token, err := BuildToken(env, userID, deviceID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if token == "" {
			t.Fatal("expected non-empty token")
		}
	})

	t.Run("missing private key", func(t *testing.T) {
		emptyEnv := mockEnv("", publicKey)
		_, err := BuildToken(emptyEnv, userID, deviceID)
		if !errors.Is(err, ErrMissingKey) {
			t.Fatalf("expected ErrMissingKey, got: %v", err)
		}
	})

	t.Run("invalid private key encoding", func(t *testing.T) {
		invalidEnv := mockEnv("invalid-base64!", publicKey)
		_, err := BuildToken(invalidEnv, userID, deviceID)
		if err == nil {
			t.Fatal("expected error for invalid key encoding")
		}
	})
}

func TestVerifyToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	env := mockEnv(privateKey, publicKey)

	userID := uuid.New()
	deviceID := "test-device-123"

	t.Run("successful token verification", func(t *testing.T) {
		token, err := BuildToken(env, userID, deviceID)
		if err != nil {
			t.Fatalf("failed to build token: %v", err)
		}

		claims, err := VerifyToken(env, token)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if claims.UserID != userID {
			t.Errorf("expected user_id %v, got %v", userID, claims.UserID)
		}
		if claims.DeviceID != deviceID {
			t.Errorf("expected device_id %s, got %s", deviceID, claims.DeviceID)
		}
	})

	t.Run("missing public key", func(t *testing.T) {
		token, _ := BuildToken(env, userID, deviceID)
		emptyEnv := mockEnv(privateKey, "")
		_, err := VerifyToken(emptyEnv, token)
		if !errors.Is(err, ErrMissingKey) {
			t.Fatalf("expected ErrMissingKey, got: %v", err)
		}
	})

	t.Run("invalid public key encoding", func(t *testing.T) {
		token, _ := BuildToken(env, userID, deviceID)
		invalidEnv := mockEnv(privateKey, "invalid-base64!")
		_, err := VerifyToken(invalidEnv, token)
		if err == nil {
			t.Fatal("expected error for invalid key encoding")
		}
	})

	t.Run("invalid token string", func(t *testing.T) {
		_, err := VerifyToken(env, "not.a.valid.token")
		if !errors.Is(err, ErrTokenInvalid) {
			t.Fatalf("expected ErrTokenInvalid, got: %v", err)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		// Create an expired token manually
		privateKeyBytes, _ := base64.StdEncoding.DecodeString(privateKey)
		privKey := ed25519.PrivateKey(privateKeyBytes)

		claims := Claims{
			UserID:   userID,
			DeviceID: deviceID,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
		expiredToken, _ := token.SignedString(privKey)

		_, err := VerifyToken(env, expiredToken)
		if !errors.Is(err, ErrTokenExpired) {
			t.Fatalf("expected ErrTokenExpired, got: %v", err)
		}
	})

	t.Run("token with wrong signing method", func(t *testing.T) {
		// Create a token signed with HS256 instead of EdDSA
		claims := Claims{
			UserID:   userID,
			DeviceID: deviceID,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		wrongMethodToken, _ := token.SignedString([]byte("secret"))

		_, err := VerifyToken(env, wrongMethodToken)
		if !errors.Is(err, ErrSigningMethod) {
			t.Fatalf("expected ErrSigningMethod, got: %v", err)
		}
	})

	t.Run("mismatched keys", func(t *testing.T) {
		// Create token with one key pair
		token, _ := BuildToken(env, userID, deviceID)

		// Try to verify with different public key
		_, differentPublicKey := generateTestKeys()
		differentEnv := mockEnv(privateKey, differentPublicKey)

		_, err := VerifyToken(differentEnv, token)
		if !errors.Is(err, ErrTokenInvalid) {
			t.Fatalf("expected ErrTokenInvalid, got: %v", err)
		}
	})
}

func TestRoundTrip(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	env := mockEnv(privateKey, publicKey)

	testCases := []struct {
		name     string
		userID   uuid.UUID
		deviceID string
	}{
		{
			name:     "standard case",
			userID:   uuid.New(),
			deviceID: "device-001",
		},
		{
			name:     "empty device id",
			userID:   uuid.New(),
			deviceID: "",
		},
		{
			name:     "long device id",
			userID:   uuid.New(),
			deviceID: "very-long-device-identifier-with-many-characters-123456789",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := BuildToken(env, tc.userID, tc.deviceID)
			if err != nil {
				t.Fatalf("BuildToken failed: %v", err)
			}

			claims, err := VerifyToken(env, token)
			if err != nil {
				t.Fatalf("VerifyToken failed: %v", err)
			}

			if claims.UserID != tc.userID {
				t.Errorf("user_id mismatch: expected %v, got %v", tc.userID, claims.UserID)
			}
			if claims.DeviceID != tc.deviceID {
				t.Errorf("device_id mismatch: expected %s, got %s", tc.deviceID, claims.DeviceID)
			}
		})
	}
}
