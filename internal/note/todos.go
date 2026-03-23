package note

import (
	"regexp"
	"slices"
	"strings"
)

// checkboxPattern matches markdown checkbox lines.
// Groups: (1) prefix like "  - " or "* ", (2) checkbox state " " or "x"/"X", (3) rest of line
var checkboxPattern = regexp.MustCompile(`^(\s*[-*]\s+)\[([ xX])\]\s+(.*)`)

// IsCheckboxLine returns true if the line is a markdown checkbox (- [ ] or - [x]).
func IsCheckboxLine(line string) bool {
	return checkboxPattern.MatchString(line)
}

// IsCheckedLine returns true if the line is a checked markdown checkbox (- [x] or - [X]).
func IsCheckedLine(line string) bool {
	m := checkboxPattern.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	return m[2] == "x" || m[2] == "X"
}

// ToggleCheckbox flips [ ] ↔ [x] on a checkbox line.
// Returns the line unchanged if it's not a checkbox.
func ToggleCheckbox(line string) string {
	m := checkboxPattern.FindStringSubmatch(line)
	if m == nil {
		return line
	}
	prefix := m[1]
	state := m[2]
	rest := m[3]

	if state == " " {
		return prefix + "[x] " + rest
	}
	return prefix + "[ ] " + rest
}

// HasUncheckedTodos returns true if the content contains at least one unchecked checkbox.
func HasUncheckedTodos(content string) bool {
	for line := range strings.SplitSeq(content, "\n") {
		if IsCheckboxLine(line) && !IsCheckedLine(line) {
			return true
		}
	}
	return false
}

// HasCheckedTodos returns true if the content contains at least one checked checkbox.
func HasCheckedTodos(content string) bool {
	return slices.ContainsFunc(strings.Split(content, "\n"), IsCheckedLine)
}

// HasAnyTodos returns true if the content contains any checkbox lines.
func HasAnyTodos(content string) bool {
	return slices.ContainsFunc(strings.Split(content, "\n"), IsCheckboxLine)
}
