# JWT

Build and verify JWTs using EdDSA (Ed25519).

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `JWT_PRIVATE_KEY` | Base64-encoded Ed25519 private key (64 bytes) | required |
| `JWT_PUBLIC_KEY` | Base64-encoded Ed25519 public key (32 bytes) | required |
| `JWT_EXPIRATION_HOURS` | Token expiration in hours | `24` |

## Generating Keys

Using Go (save as `gen.go` and run with `go run gen.go`):

```go
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func main() {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	fmt.Println("JWT_PRIVATE_KEY=" + base64.StdEncoding.EncodeToString(priv))
	fmt.Println("JWT_PUBLIC_KEY=" + base64.StdEncoding.EncodeToString(pub))
}
```

Using openssl:

```bash
# Generate a single key and derive both values from it
openssl genpkey -algorithm ed25519 -outform PEM -out /tmp/ed25519.pem
# Extract 32-byte seed (private) and 32-byte public key
openssl pkey -in /tmp/ed25519.pem -outform DER 2>/dev/null | tail -c 32 > /tmp/ed25519_seed
openssl pkey -in /tmp/ed25519.pem -pubout -outform DER 2>/dev/null | tail -c 32 > /tmp/ed25519_pub
# JWT_PRIVATE_KEY is seed+pub (64 bytes), JWT_PUBLIC_KEY is pub (32 bytes)
echo "JWT_PRIVATE_KEY=$(cat /tmp/ed25519_seed /tmp/ed25519_pub | base64)"
echo "JWT_PUBLIC_KEY=$(cat /tmp/ed25519_pub | base64)"
rm /tmp/ed25519.pem /tmp/ed25519_seed /tmp/ed25519_pub
```
