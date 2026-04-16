// Package dateparse provides date parsing for the ruin CLI.
package dateparse

import (
	"fmt"
	"strings"
	"time"
)

// DateRange represents a range of time (inclusive of start, exclusive of end).
type DateRange struct {
	Start time.Time
	End   time.Time
}

// Contains returns true if the given time falls within the range.
// The range is [Start, End) - inclusive of Start, exclusive of End.
func (r DateRange) Contains(t time.Time) bool {
	return !t.Before(r.Start) && t.Before(r.End)
}

// Parse parses a date string and returns the corresponding DateRange.
// Supports:
//   - Exact dates: YYYY-MM-DD, YYYY-MM, YYYY
//   - Simple helpers: today, yesterday, tomorrow
func Parse(s string) (DateRange, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	now := time.Now()

	if r, ok := parseNaturalLanguage(s, now); ok {
		return r, nil
	}

	if r, ok := parseISODate(s); ok {
		return r, nil
	}

	return DateRange{}, fmt.Errorf("unrecognized date format: %s", s)
}

// ParseWithReference parses a date string relative to a reference time.
func ParseWithReference(s string, ref time.Time) (DateRange, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	if r, ok := parseNaturalLanguage(s, ref); ok {
		return r, nil
	}

	if r, ok := parseISODate(s); ok {
		return r, nil
	}

	return DateRange{}, fmt.Errorf("unrecognized date format: %s", s)
}

func parseNaturalLanguage(s string, now time.Time) (DateRange, bool) {
	today := startOfDay(now)

	switch s {
	case "today":
		return DateRange{
			Start: today,
			End:   today.AddDate(0, 0, 1),
		}, true

	case "yesterday":
		yesterday := today.AddDate(0, 0, -1)
		return DateRange{
			Start: yesterday,
			End:   today,
		}, true

	case "tomorrow":
		tomorrow := today.AddDate(0, 0, 1)
		return DateRange{
			Start: tomorrow,
			End:   tomorrow.AddDate(0, 0, 1),
		}, true

	case "this-week":
		// time.Weekday uses Sunday=0; remap so Monday=1..Sunday=7 to get ISO week start.
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := today.AddDate(0, 0, -(weekday - 1))
		return DateRange{Start: monday, End: monday.AddDate(0, 0, 7)}, true

	case "last-week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := today.AddDate(0, 0, -(weekday-1)-7)
		return DateRange{Start: monday, End: monday.AddDate(0, 0, 7)}, true

	case "next-week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := today.AddDate(0, 0, -(weekday-1)+7)
		return DateRange{Start: monday, End: monday.AddDate(0, 0, 7)}, true

	case "this-month":
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return DateRange{Start: first, End: first.AddDate(0, 1, 0)}, true

	case "last-month":
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -1, 0)
		return DateRange{Start: first, End: first.AddDate(0, 1, 0)}, true

	case "next-month":
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0)
		return DateRange{Start: first, End: first.AddDate(0, 1, 0)}, true
	}

	return DateRange{}, false
}

func parseISODate(s string) (DateRange, bool) {
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return DateRange{
			Start: t,
			End:   t.AddDate(0, 0, 1),
		}, true
	}

	if t, err := time.ParseInLocation("2006-01", s, time.Local); err == nil {
		return DateRange{
			Start: t,
			End:   t.AddDate(0, 1, 0),
		}, true
	}

	if len(s) == 4 {
		if t, err := time.ParseInLocation("2006", s, time.Local); err == nil {
			return DateRange{
				Start: t,
				End:   t.AddDate(1, 0, 0),
			}, true
		}
	}

	return DateRange{}, false
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// Today returns the DateRange for today.
func Today() DateRange {
	r, _ := Parse("today")
	return r
}

// Yesterday returns the DateRange for yesterday.
func Yesterday() DateRange {
	r, _ := Parse("yesterday")
	return r
}
