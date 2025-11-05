package timehelpers

import "time"

// Return "now" as UTC trunctated by milliseconds
func Now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
