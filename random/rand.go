package random

import (
	"crypto/rand"
	"fmt"
	"math/big"
	insecure "math/rand"
)

const AlphabeticLetters = "abcdefghijklmnopqrstuvwxyz"

// Insecure version (not cryptographically strong) that generates a random string using alphabetic letters
func RandomString(n int) string {
	result := make([]byte, n)
	for i := range result {
		result[i] = AlphabeticLetters[insecure.Intn(len(AlphabeticLetters))]
	}
	return string(result)
}

// Secure version that generates a random string using provided letters
func RandomSecureString(letters string, n int) (string, error) {
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random secure int: %w", err)
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret), nil
}
