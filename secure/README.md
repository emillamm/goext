# Secure

Library for encrypting and decrypting data using AES-256-GCM symmetric encryption.

## Usage

```go
import (
    "os"
    "github.com/emillamm/goext/secure"
)

// Encrypt bytes
plaintext := []byte("secret data")
ciphertext, err := secure.EncryptSymmetric(os.Getenv, plaintext)

// Decrypt bytes
decrypted, err := secure.DecryptSymmetric(os.Getenv, ciphertext)

// Encrypt string (returns base64-encoded ciphertext)
ciphertextStr, err := secure.EncryptSymmetricString(os.Getenv, "secret message")

// Decrypt string
plaintext, err := secure.DecryptSymmetricString(os.Getenv, ciphertextStr)
```

## Environment Variable

Set `SECURE_KEY` as a base64-encoded 32-byte (256-bit) key:

```bash
export SECURE_KEY="your-base64-encoded-key-here"
```

## Generate Random Key

Generate a secure random 256-bit key and encode it as base64:

```bash
openssl rand -base64 32
```
