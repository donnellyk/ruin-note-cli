package dateparse

import (
	"strconv"
	"strings"
	"time"
)

// ResolveDate resolves a date token (without @ prefix) to a single date.
// Returns the resolved date and true if recognized, or zero time and false
// if the token is not a valid date expression.
//
// For shared terms (today, tomorrow, next-week, monday, etc.), this calls
// ParseWithReference and returns .Start.
//
// For numeric offsets (2-days, 3-weeks, etc.), this applies FORWARD direction
// (today + N), unlike Parse() which treats them as lookback.
func ResolveDate(token string) (time.Time, bool) {
	return ResolveDateAt(token, time.Now())
}

// ResolveDateAt resolves a date token relative to a reference time.
// This is the testable version of ResolveDate.
func ResolveDateAt(token string, ref time.Time) (time.Time, bool) {
	token = strings.TrimSpace(strings.ToLower(token))
	today := startOfDay(ref)

	// Try forward-looking numeric offsets first (before Parse, which treats them as lookback)
	if t, ok := resolveForwardOffset(token, today); ok {
		return t, true
	}

	// Try shared terms via ParseWithReference
	r, err := ParseWithReference(token, ref)
	if err != nil {
		return time.Time{}, false
	}
	return r.Start, true
}

// resolveForwardOffset handles numeric offset tokens with forward semantics.
// "2-days" → today + 2 days, "3-weeks" → today + 21 days, etc.
func resolveForwardOffset(token string, today time.Time) (time.Time, bool) {
	// Days: 2-days, 2-day
	if matches := daysPattern.FindStringSubmatch(token); len(matches) == 2 {
		// Skip shorthand forms (7d) — those are lookback-only in Parse()
		// Only handle explicit forms (7-days, 7-day) for forward resolution
		if !strings.Contains(token, "-") {
			return time.Time{}, false
		}
		days, _ := strconv.Atoi(matches[1])
		return today.AddDate(0, 0, days), true
	}

	// Weeks: 3-weeks, 3-week
	if matches := weeksPattern.FindStringSubmatch(token); len(matches) == 2 {
		if !strings.Contains(token, "-") {
			return time.Time{}, false
		}
		weeks, _ := strconv.Atoi(matches[1])
		return today.AddDate(0, 0, weeks*7), true
	}

	// Months: 2-months, 2-month
	if matches := monthsPattern.FindStringSubmatch(token); len(matches) == 2 {
		if !strings.Contains(token, "-") {
			return time.Time{}, false
		}
		months, _ := strconv.Atoi(matches[1])
		return today.AddDate(0, months, 0), true
	}

	// Years: 2-years, 2-year
	if matches := yearsPattern.FindStringSubmatch(token); len(matches) == 2 {
		if !strings.Contains(token, "-") {
			return time.Time{}, false
		}
		years, _ := strconv.Atoi(matches[1])
		return today.AddDate(years, 0, 0), true
	}

	return time.Time{}, false
}
