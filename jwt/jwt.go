package jwt

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"time"

	"github.com/emillamm/envx"
	"github.com/emillamm/goext/uuid"
	"github.com/golang-jwt/jwt/v5"
)

// Sentinel errors for JWT verification failures
var (
	ErrTokenExpired  = errors.New("token has expired")
	ErrTokenInvalid  = errors.New("token is invalid")
	ErrSigningMethod = errors.New("unexpected signing method")
	ErrMissingKey    = errors.New("missing signing key in environment")
)

// Claims represents the custom JWT claims containing user and device information
type Claims struct {
	UserID   uuid.UUID `json:"user_id"`
	DeviceID string    `json:"device_id"`
	jwt.RegisteredClaims
}

// BuildToken creates a new JWT token with the provided user_id and device_id
// The token is signed using EdDSA with the private key from environment variables
// Expiration time is read from JWT_EXPIRATION_HOURS (defaults to 24 hours)
func BuildToken(env envx.EnvX, userID uuid.UUID, deviceID string) (string, error) {
	// Get private key from environment
	privateKeyB64 := env("JWT_PRIVATE_KEY")
	if privateKeyB64 == "" {
		return "", ErrMissingKey
	}

	// Decode the base64-encoded private key
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return "", errors.New("failed to decode private key: " + err.Error())
	}

	privateKey := ed25519.PrivateKey(privateKeyBytes)

	// Get expiration hours from environment (default: 24 hours)
	expirationHours, err := env.Int("JWT_EXPIRATION_HOURS").Default(24)
	if err != nil {
		return "", errors.New("failed to parse JWT_EXPIRATION_HOURS: " + err.Error())
	}

	now := time.Now().UTC()

	// Create claims
	claims := Claims{
		UserID:   userID,
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expirationHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)

	// Sign and return the token
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", errors.New("failed to sign token: " + err.Error())
	}

	return tokenString, nil
}

// VerifyToken verifies a JWT token string and returns the claims if valid
// Returns sentinel errors for specific failure cases
func VerifyToken(env envx.EnvX, tokenString string) (*Claims, error) {
	// Get public key from environment
	publicKeyB64 := env("JWT_PUBLIC_KEY")
	if publicKeyB64 == "" {
		return nil, ErrMissingKey
	}

	// Decode the base64-encoded public key
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return nil, errors.New("failed to decode public key: " + err.Error())
	}

	publicKey := ed25519.PublicKey(publicKeyBytes)

	// Parse and verify token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, ErrSigningMethod
		}
		return publicKey, nil
	})
	if err != nil {
		// Check for specific error types
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, ErrSigningMethod) {
			return nil, ErrSigningMethod
		}
		return nil, ErrTokenInvalid
	}

	// Extract and return claims
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrTokenInvalid
}
