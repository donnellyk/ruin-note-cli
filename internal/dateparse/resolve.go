package dateparse

import (
	"strings"
	"time"
)

// ResolveDate resolves a date token (without @ prefix) to a single date.
// Returns the resolved date and true if recognized, or zero time and false
// if the token is not a valid date expression.
//
// Supports: today, tomorrow, yesterday, YYYY-MM-DD.
func ResolveDate(token string) (time.Time, bool) {
	return ResolveDateAt(token, time.Now())
}

// ResolveDateAt resolves a date token relative to a reference time.
func ResolveDateAt(token string, ref time.Time) (time.Time, bool) {
	token = strings.TrimSpace(strings.ToLower(token))

	r, err := ParseWithReference(token, ref)
	if err != nil {
		return time.Time{}, false
	}
	return r.Start, true
}
