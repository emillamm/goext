package uuid

import (
	"testing"

	guuid "github.com/google/uuid"
)

func TestNew(t *testing.T) {
	u := New()
	if guuid.UUID(u) == (guuid.UUID{}) {
		t.Fatal("expected non-zero UUID")
	}
}

func TestString(t *testing.T) {
	u := New()
	s := u.String()
	if len(s) != 22 {
		t.Fatalf("expected 22-character shortuuid, got %d characters", len(s))
	}
}

func TestParse(t *testing.T) {
	t.Run("shortuuid format", func(t *testing.T) {
		original := New()
		shortStr := original.String()

		parsed, err := Parse(shortStr)
		if err != nil {
			t.Fatalf("failed to parse shortuuid: %v", err)
		}
		if parsed.GUUID() != original.GUUID() {
			t.Errorf("expected %v, got %v", original.GUUID(), parsed.GUUID())
		}
	})

	t.Run("google uuid format", func(t *testing.T) {
		original := New()
		googleStr := original.GUUID().String()

		parsed, err := Parse(googleStr)
		if err != nil {
			t.Fatalf("failed to parse google uuid: %v", err)
		}
		if parsed.GUUID() != original.GUUID() {
			t.Errorf("expected %v, got %v", original.GUUID(), parsed.GUUID())
		}
	})

	t.Run("invalid length", func(t *testing.T) {
		_, err := Parse("invalid")
		if err == nil {
			t.Fatal("expected error for invalid length")
		}
	})
}

func TestMustParse(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		u := New()
		result := MustParse(u.String())
		if result.GUUID() != u.GUUID() {
			t.Errorf("expected %v, got %v", u.GUUID(), result.GUUID())
		}
	})

	t.Run("invalid input panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for invalid input")
			}
		}()
		MustParse("invalid")
	})
}

func TestValueAndScan(t *testing.T) {
	original := New()

	val, err := original.Value()
	if err != nil {
		t.Fatalf("Value() failed: %v", err)
	}

	var scanned UUID
	if err := scanned.Scan(val); err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	if scanned.GUUID() != original.GUUID() {
		t.Errorf("expected %v, got %v", original.GUUID(), scanned.GUUID())
	}
}

func TestScanErrors(t *testing.T) {
	var u UUID

	t.Run("nil value", func(t *testing.T) {
		if err := u.Scan(nil); err == nil {
			t.Fatal("expected error for nil")
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		if err := u.Scan(123); err == nil {
			t.Fatal("expected error for invalid type")
		}
	})

	t.Run("byte slice input", func(t *testing.T) {
		original := New()
		if err := u.Scan([]byte(original.String())); err != nil {
			t.Fatalf("Scan([]byte) failed: %v", err)
		}
		if u.GUUID() != original.GUUID() {
			t.Errorf("expected %v, got %v", original.GUUID(), u.GUUID())
		}
	})
}
