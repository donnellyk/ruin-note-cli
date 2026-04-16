package note

import (
	"regexp"
	"slices"
	"strings"
)

// checkboxPattern groups: (1) prefix like "  - ", (2) state " "/"x"/"X", (3) rest.
var checkboxPattern = regexp.MustCompile(`^(\s*[-*]\s+)\[([ xX])\]\s+(.*)`)

func IsCheckboxLine(line string) bool {
	return checkboxPattern.MatchString(line)
}

func IsCheckedLine(line string) bool {
	m := checkboxPattern.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	return m[2] == "x" || m[2] == "X"
}

// ToggleCheckbox flips [ ] <-> [x]. Returns the line unchanged if not a checkbox.
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

func HasUncheckedTodos(content string) bool {
	for line := range strings.SplitSeq(content, "\n") {
		if IsCheckboxLine(line) && !IsCheckedLine(line) {
			return true
		}
	}
	return false
}

func HasCheckedTodos(content string) bool {
	return slices.ContainsFunc(strings.Split(content, "\n"), IsCheckedLine)
}

func HasAnyTodos(content string) bool {
	return slices.ContainsFunc(strings.Split(content, "\n"), IsCheckboxLine)
}
