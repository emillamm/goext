package uuid

import (
	"database/sql/driver"
	"fmt"

	guuid "github.com/google/uuid"
	"github.com/lithammer/shortuuid/v4"
)

// UUID wraps google's uuid.UUID and provides serialization using shortuuid encoding.
// It stores the UUID internally but encodes/decodes to a 22-character base57 string for database I/O.
type UUID guuid.UUID

// New generates a new random UUID.
func New() UUID {
	return UUID(guuid.New())
}

// String returns the shortuuid-encoded string representation (22 characters).
func (u UUID) String() string {
	return shortuuid.DefaultEncoder.Encode(guuid.UUID(u))
}

func (u UUID) Underlying() guuid.UUID {
	return guuid.UUID(u)
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
