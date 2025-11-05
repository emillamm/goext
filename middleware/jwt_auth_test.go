package middleware

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/emillamm/goext/jwt"
	jwtlib "github.com/golang-jwt/jwt/v5"
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

// createTestHandler returns a handler that checks context values
func createTestHandler(t *testing.T, expectedUserID uuid.UUID, expectedDeviceID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := GetUserID(r.Context())
		if err != nil {
			t.Errorf("expected user ID in context: %v", err)
		}
		if userID != expectedUserID {
			t.Errorf("expected user ID %v, got %v", expectedUserID, userID)
		}

		deviceID, err := GetDeviceID(r.Context())
		if err != nil {
			t.Errorf("expected device ID in context: %v", err)
		}
		if deviceID != expectedDeviceID {
			t.Errorf("expected device ID %s, got %s", expectedDeviceID, deviceID)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// dummy error logger
func errorLog(err error) {}

func TestJWTAuth_SuccessfulAuthentication(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	env := mockEnv(privateKey, publicKey)

	userID := uuid.New()
	deviceID := "test-device-123"

	// Create valid token
	tokenString, err := jwt.BuildToken(env, userID, deviceID)
	if err != nil {
		t.Fatalf("failed to build token: %v", err)
	}

	// Create test handler
	handler := JWTAuth(env, errorLog)(createTestHandler(t, userID, deviceID))

	// Create request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	// Record response
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Check X-Auth-Token-Expiry header is set
	expiryHeader := rr.Header().Get("X-Auth-Token-Expiry")
	if expiryHeader == "" {
		t.Error("expected X-Auth-Token-Expiry header to be set")
	}

	// Verify expiry value is reasonable (should be in the future)
	expiryMillis, err := strconv.ParseInt(expiryHeader, 10, 64)
	if err != nil {
		t.Errorf("failed to parse expiry header: %v", err)
	}
	if expiryMillis <= time.Now().UnixMilli() {
		t.Error("expiry time should be in the future")
	}
}

func TestJWTAuth_MissingAuthorizationHeader(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	env := mockEnv(privateKey, publicKey)

	handler := JWTAuth(env, errorLog)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	// X-Auth-Token-Expiry header should NOT be set
	if rr.Header().Get("X-Auth-Token-Expiry") != "" {
		t.Error("X-Auth-Token-Expiry header should not be set for missing auth header")
	}
}

func TestJWTAuth_InvalidAuthorizationFormat(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	env := mockEnv(privateKey, publicKey)

	handler := JWTAuth(env, errorLog)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	testCases := []struct {
		name   string
		header string
	}{
		{"no bearer prefix", "token123"},
		{"wrong prefix", "Basic token123"},
		{"only bearer", "Bearer"},
		{"empty bearer", "Bearer "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tc.header)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", rr.Code)
			}

			// X-Auth-Token-Expiry header should NOT be set
			if rr.Header().Get("X-Auth-Token-Expiry") != "" {
				t.Error("X-Auth-Token-Expiry header should not be set for invalid format")
			}
		})
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	env := mockEnv(privateKey, publicKey)

	handler := JWTAuth(env, errorLog)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.string")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	// X-Auth-Token-Expiry header should NOT be set
	if rr.Header().Get("X-Auth-Token-Expiry") != "" {
		t.Error("X-Auth-Token-Expiry header should not be set for invalid token")
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	env := mockEnv(privateKey, publicKey)

	// Create an expired token
	privateKeyBytes, _ := base64.StdEncoding.DecodeString(privateKey)
	privKey := ed25519.PrivateKey(privateKeyBytes)

	userID := uuid.New()
	deviceID := "test-device-123"

	claims := jwt.Claims{
		UserID:   userID,
		DeviceID: deviceID,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodEdDSA, claims)
	expiredToken, _ := token.SignedString(privKey)

	handler := JWTAuth(env, errorLog)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	// X-Auth-Token-Expiry header should be set to 0
	expiryHeader := rr.Header().Get("X-Auth-Token-Expiry")
	if expiryHeader != "0" {
		t.Errorf("expected X-Auth-Token-Expiry to be '0', got '%s'", expiryHeader)
	}
}

func TestJWTAuth_WrongSigningMethod(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	env := mockEnv(privateKey, publicKey)

	// Create a token with HS256 instead of EdDSA
	userID := uuid.New()
	deviceID := "test-device-123"

	claims := jwt.Claims{
		UserID:   userID,
		DeviceID: deviceID,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	wrongMethodToken, _ := token.SignedString([]byte("secret"))

	handler := JWTAuth(env, errorLog)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+wrongMethodToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	// X-Auth-Token-Expiry header should NOT be set
	if rr.Header().Get("X-Auth-Token-Expiry") != "" {
		t.Error("X-Auth-Token-Expiry header should not be set for wrong signing method")
	}
}

func TestJWTAuth_MismatchedKeys(t *testing.T) {
	privateKey, _ := generateTestKeys()
	_, differentPublicKey := generateTestKeys()

	// Build token with one key, verify with different key
	buildEnv := mockEnv(privateKey, "")
	verifyEnv := mockEnv(privateKey, differentPublicKey)

	userID := uuid.New()
	deviceID := "test-device-123"

	tokenString, _ := jwt.BuildToken(buildEnv, userID, deviceID)

	handler := JWTAuth(verifyEnv, errorLog)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	// X-Auth-Token-Expiry header should NOT be set
	if rr.Header().Get("X-Auth-Token-Expiry") != "" {
		t.Error("X-Auth-Token-Expiry header should not be set for mismatched keys")
	}
}

func TestJWTAuth_MissingPublicKey(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	buildEnv := mockEnv(privateKey, publicKey)
	verifyEnv := mockEnv(privateKey, "") // Missing public key

	userID := uuid.New()
	deviceID := "test-device-123"

	tokenString, _ := jwt.BuildToken(buildEnv, userID, deviceID)

	handler := JWTAuth(verifyEnv, errorLog)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}

	// X-Auth-Token-Expiry header should NOT be set
	if rr.Header().Get("X-Auth-Token-Expiry") != "" {
		t.Error("X-Auth-Token-Expiry header should not be set for server errors")
	}
}

func TestJWTAuth_BearerCaseInsensitive(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	env := mockEnv(privateKey, publicKey)

	userID := uuid.New()
	deviceID := "test-device-123"

	tokenString, err := jwt.BuildToken(env, userID, deviceID)
	if err != nil {
		t.Fatalf("failed to build token: %v", err)
	}

	testCases := []string{
		"Bearer " + tokenString,
		"bearer " + tokenString,
		"BEARER " + tokenString,
		"BeArEr " + tokenString,
	}

	for _, authHeader := range testCases {
		t.Run(authHeader[:6], func(t *testing.T) {
			handler := JWTAuth(env, errorLog)(createTestHandler(t, userID, deviceID))

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", authHeader)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rr.Code)
			}
		})
	}
}

func TestGetUserID_NotInContext(t *testing.T) {
	ctx := context.Background()
	_, err := GetUserID(ctx)
	if !errors.Is(err, ErrNoUserId) {
		t.Error("expected GetUserID to return error ErrNoUserId for empty context")
	}
}

func TestGetDeviceID_NotInContext(t *testing.T) {
	ctx := context.Background()
	_, err := GetDeviceID(ctx)
	if !errors.Is(err, ErrNoDeviceId) {
		t.Error("expected GetDeviceID to return error ErrNoDeviceId for empty context")
	}
}

func TestGetUserID_InContext(t *testing.T) {
	expectedUserID := uuid.New()
	ctx := context.WithValue(context.Background(), userIDKey, expectedUserID)

	userID, err := GetUserID(ctx)
	if err != nil {
		t.Errorf("expected GetUserID to return no error, but it returned: %v", err)
	}
	if userID != expectedUserID {
		t.Errorf("expected user ID %v, got %v", expectedUserID, userID)
	}
}

func TestGetDeviceID_InContext(t *testing.T) {
	expectedDeviceID := "device-123"
	ctx := context.WithValue(context.Background(), deviceIDKey, expectedDeviceID)

	deviceID, err := GetDeviceID(ctx)
	if err != nil {
		t.Errorf("expected GetDeviceID to return no error, but it returned: %v", err)
	}
	if deviceID != expectedDeviceID {
		t.Errorf("expected device ID %s, got %s", expectedDeviceID, deviceID)
	}
}
