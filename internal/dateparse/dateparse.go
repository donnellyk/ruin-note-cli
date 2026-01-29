// Package dateparse provides natural language date parsing for the ruin CLI.
package dateparse

import (
	"fmt"
	"regexp"
	"strconv"
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
// Supports various formats:
//   - Exact dates: YYYY-MM-DD, YYYY-MM, YYYY
//   - Natural language: today, yesterday, tomorrow
//   - Relative: this-week, last-week, this-month, last-month, this-year
//   - Duration: 7d, 7-days, 2w, 2-weeks, 3m, 3-months
func Parse(s string) (DateRange, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	now := time.Now()

	// Try natural language first
	if r, ok := parseNaturalLanguage(s, now); ok {
		return r, nil
	}

	// Try relative duration (7d, 2w, etc.)
	if r, ok := parseRelativeDuration(s, now); ok {
		return r, nil
	}

	// Try ISO date formats
	if r, ok := parseISODate(s); ok {
		return r, nil
	}

	return DateRange{}, fmt.Errorf("unrecognized date format: %s", s)
}

// ParseWithReference parses a date string relative to a reference time.
// This is useful for testing or when you need to parse relative to a specific time.
func ParseWithReference(s string, ref time.Time) (DateRange, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	// Try natural language first
	if r, ok := parseNaturalLanguage(s, ref); ok {
		return r, nil
	}

	// Try relative duration (7d, 2w, etc.)
	if r, ok := parseRelativeDuration(s, ref); ok {
		return r, nil
	}

	// Try ISO date formats
	if r, ok := parseISODate(s); ok {
		return r, nil
	}

	return DateRange{}, fmt.Errorf("unrecognized date format: %s", s)
}

// parseNaturalLanguage handles natural language date expressions.
func parseNaturalLanguage(s string, now time.Time) (DateRange, bool) {
	// Get start of today (midnight local time)
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
		// Monday through Sunday
		weekStart := startOfWeek(today)
		return DateRange{
			Start: weekStart,
			End:   weekStart.AddDate(0, 0, 7),
		}, true

	case "last-week":
		weekStart := startOfWeek(today).AddDate(0, 0, -7)
		return DateRange{
			Start: weekStart,
			End:   weekStart.AddDate(0, 0, 7),
		}, true

	case "this-month":
		monthStart := startOfMonth(today)
		return DateRange{
			Start: monthStart,
			End:   monthStart.AddDate(0, 1, 0),
		}, true

	case "last-month":
		thisMonth := startOfMonth(today)
		lastMonth := thisMonth.AddDate(0, -1, 0)
		return DateRange{
			Start: lastMonth,
			End:   thisMonth,
		}, true

	case "this-year":
		yearStart := startOfYear(today)
		return DateRange{
			Start: yearStart,
			End:   yearStart.AddDate(1, 0, 0),
		}, true

	case "last-year":
		thisYear := startOfYear(today)
		lastYear := thisYear.AddDate(-1, 0, 0)
		return DateRange{
			Start: lastYear,
			End:   thisYear,
		}, true
	}

	return DateRange{}, false
}

// Patterns for relative durations
var (
	daysPattern   = regexp.MustCompile(`^(\d+)(?:d|-days?)$`)
	weeksPattern  = regexp.MustCompile(`^(\d+)(?:w|-weeks?)$`)
	monthsPattern = regexp.MustCompile(`^(\d+)(?:m|-months?)$`)
)

// parseRelativeDuration handles duration expressions like "7d", "2-weeks", etc.
// These return a range from N units ago to now.
func parseRelativeDuration(s string, now time.Time) (DateRange, bool) {
	today := startOfDay(now)
	endOfToday := today.AddDate(0, 0, 1)

	// Days: 7d, 7-days, 7-day
	if matches := daysPattern.FindStringSubmatch(s); len(matches) == 2 {
		days, _ := strconv.Atoi(matches[1])
		start := today.AddDate(0, 0, -days+1) // Include today, go back N-1 more days
		return DateRange{Start: start, End: endOfToday}, true
	}

	// Weeks: 2w, 2-weeks, 2-week
	if matches := weeksPattern.FindStringSubmatch(s); len(matches) == 2 {
		weeks, _ := strconv.Atoi(matches[1])
		start := today.AddDate(0, 0, -weeks*7+1)
		return DateRange{Start: start, End: endOfToday}, true
	}

	// Months: 3m, 3-months, 3-month
	if matches := monthsPattern.FindStringSubmatch(s); len(matches) == 2 {
		months, _ := strconv.Atoi(matches[1])
		start := today.AddDate(0, -months, 1)
		return DateRange{Start: start, End: endOfToday}, true
	}

	return DateRange{}, false
}

// parseISODate handles ISO-style date formats.
func parseISODate(s string) (DateRange, bool) {
	// Full date: YYYY-MM-DD
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return DateRange{
			Start: t,
			End:   t.AddDate(0, 0, 1),
		}, true
	}

	// Year and month: YYYY-MM
	if t, err := time.ParseInLocation("2006-01", s, time.Local); err == nil {
		return DateRange{
			Start: t,
			End:   t.AddDate(0, 1, 0),
		}, true
	}

	// Year only: YYYY
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

// Helper functions for date calculations

// startOfDay returns midnight of the given day in local time.
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// startOfWeek returns midnight of the Monday of the given week.
func startOfWeek(t time.Time) time.Time {
	day := startOfDay(t)
	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday is the 7th day
	}
	return day.AddDate(0, 0, -(weekday - 1))
}

// startOfMonth returns midnight of the first day of the given month.
func startOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

// startOfYear returns midnight of the first day of the given year.
func startOfYear(t time.Time) time.Time {
	y, _, _ := t.Date()
	return time.Date(y, 1, 1, 0, 0, 0, 0, t.Location())
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
