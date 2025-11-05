package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/emillamm/envx"
	"github.com/emillamm/goext/jwt"
	"github.com/google/uuid"
)

// contextKey is an unexported type for context keys to avoid collisions
type contextKey string

const (
	userIDKey   contextKey = "user_id"
	deviceIDKey contextKey = "device_id"
)

var (
	ErrNoUserId   = errors.New("user_id is not attached to context")
	ErrNoDeviceId = errors.New("device_id is not attached to context")
)

// JWTAuth is a middleware that extracts and verifies JWT tokens from the Authorization header.
//
// Behavior:
//   - Extracts JWT token from Authorization header (expects "Bearer <token>" format)
//   - Verifies the token using jwt.VerifyToken
//   - On success: adds user_id and device_id to request context and sets X-Auth-Token-Expiry header
//     with the token's expiration time as a Unix timestamp in milliseconds
//   - On expired token: returns 401 Unauthorized and sets X-Auth-Token-Expiry header to 0
//   - On other failures (missing, invalid, or malformed token): returns 401 Unauthorized
//     without setting the X-Auth-Token-Expiry header
//
// HTTP Status Codes:
//   - 401 Unauthorized: Missing, invalid, expired, or malformed token
//   - 500 Internal Server Error: Server-side verification errors
//
// Use GetUserID and GetDeviceID helper functions to extract claims from the request context.
func JWTAuth(env envx.EnvX, errorLog func(error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			// Check for Bearer token format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// Verify token
			claims, err := jwt.VerifyToken(env, tokenString)
			if err != nil {
				// Handle specific error cases
				if errors.Is(err, jwt.ErrTokenExpired) {
					// Set expiry header to 0 for expired tokens
					w.Header().Set("X-Auth-Token-Expiry", "0")
					http.Error(w, "Token has expired", http.StatusUnauthorized)
					return
				}
				if errors.Is(err, jwt.ErrMissingKey) {
					errorLog(err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				// For invalid tokens, signing method errors, and other errors
				// Do not set X-Auth-Token-Expiry header
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Set expiry header with Unix timestamp in milliseconds
			expiryMillis := claims.ExpiresAt.Time.UnixMilli()
			w.Header().Set("X-Auth-Token-Expiry", strconv.FormatInt(expiryMillis, 10))

			// Add claims to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, deviceIDKey, claims.DeviceID)

			// Call next handler with updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the user ID from the request context.
// Returns the user ID and true if found, otherwise returns zero UUID and false.
func GetUserID(ctx context.Context) (userId uuid.UUID, err error) {
	userId, ok := ctx.Value(userIDKey).(uuid.UUID)
	if !ok {
		err = ErrNoUserId
	}
	return
}

// GetDeviceID extracts the device ID from the request context.
// Returns the device ID and true if found, otherwise returns empty string and false.
func GetDeviceID(ctx context.Context) (deviceId string, err error) {
	deviceId, ok := ctx.Value(deviceIDKey).(string)
	if !ok {
		err = ErrNoDeviceId
	}
	return
}
