package uuid

import (
	"bytes"
	"database/sql/driver"
	"fmt"
	"time"

	guuid "github.com/google/uuid"
	"github.com/lithammer/shortuuid/v4"
)

// UUID wraps google's uuid.UUID and provides serialization using shortuuid encoding.
// It is a type definition of google's uuid.UUID but encodes/decodes to a 22-character base57 string for database I/O.
type UUID guuid.UUID

// Nil type (all zeros)
var Nil UUID

// New generates a new random UUID (v4).
func New() UUID {
	return UUID(guuid.New())
}

// NewV5 generates a deterministic UUID (v5) from a namespace and data using SHA-1.
// The same (namespace, data) always produces the same UUID.
func NewV5(namespace UUID, data []byte) UUID {
	return UUID(guuid.NewSHA1(guuid.UUID(namespace), data))
}

// NewV7 generates a new time-ordered UUID (v7).
// The timestamp is embedded in the most significant bits, so UUIDs generated
// later are lexicographically greater in both raw and shortuuid-encoded form.
func NewV7() UUID {
	u, err := guuid.NewV7()
	if err != nil {
		panic(fmt.Sprintf("failed to generate UUIDv7: %v", err))
	}
	return UUID(u)
}

// String returns the shortuuid-encoded string representation (22 characters).
func (u UUID) String() string {
	return shortuuid.DefaultEncoder.Encode(guuid.UUID(u))
}

func (u UUID) Underlying() guuid.UUID {
	return guuid.UUID(u)
}

// Compare returns -1 if u < other, 0 if u == other, and +1 if u > other,
// ordered by the raw 16-byte value. For time-ordered v7 UUIDs this is
// equivalent to comparing their timestamps, with the trailing random bits
// breaking ties between UUIDs generated in the same millisecond.
func (u UUID) Compare(other UUID) int {
	a, b := guuid.UUID(u), guuid.UUID(other)
	return bytes.Compare(a[:], b[:])
}

// Before reports whether u was generated before other. It is only meaningful
// for time-ordered UUIDs (e.g. v7); see Compare.
func (u UUID) Before(other UUID) bool {
	return u.Compare(other) < 0
}

// After reports whether u was generated after other. It is only meaningful
// for time-ordered UUIDs (e.g. v7); see Compare.
func (u UUID) After(other UUID) bool {
	return u.Compare(other) > 0
}

// Time returns the timestamp embedded in the UUID.
//
// Like google/uuid's Time, the returned value is only meaningful for
// time-based UUIDs (v1, v2, v6, v7). Calling it on a random (v4) or hashed
// (v5) UUID does not error or panic; it reinterprets the leading bytes as a
// timestamp and returns a meaningless value. It is the caller's
// responsibility to know the UUID is time-based.
func (u UUID) Time() time.Time {
	sec, nsec := guuid.UUID(u).Time().UnixTime()
	return time.Unix(sec, nsec)
}

// Value implements driver.Valuer for database writes.
// Encodes the UUID as a 22-character shortuuid string.
func (u UUID) Value() (driver.Value, error) {
	return u.String(), nil
}

// Scan implements sql.Scanner for database reads.
// Parses a 22-character shortuuid or 36-character google uuid string back into a UUID.
func (u *UUID) Scan(src interface{}) error {
	if src == nil {
		return fmt.Errorf("cannot scan nil into a UUID")
	}

	var str string
	switch v := src.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan type %T into UUID", src)
	}

	parsed, err := Parse(str)
	if err != nil {
		return err
	}
	*u = UUID(parsed)
	return nil
}

// Parses a 22-character shortuuid or 36-character google uuid string back into a UUID.
func Parse(s string) (uuid UUID, err error) {
	var decoded guuid.UUID
	if len(s) == 22 {
		decoded, err = shortuuid.DefaultEncoder.Decode(s)
	} else if len(s) == 36 {
		decoded, err = guuid.Parse(s)
	} else {
		err = fmt.Errorf("%d is not a valid length", len(s))
	}
	if err != nil {
		err = fmt.Errorf("failed to parse uuid: %w", err)
		return
	}

	uuid = UUID(decoded)
	return
}

// MustParse parses a shortuuid string into a UUID, panicking on error.
func MustParse(s string) UUID {
	uuid, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return uuid
}
