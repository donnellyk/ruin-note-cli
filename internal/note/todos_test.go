package note

import "testing"

func TestIsCheckboxLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"- [ ] unchecked item", true},
		{"- [x] checked item", true},
		{"- [X] checked uppercase", true},
		{"* [ ] asterisk bullet", true},
		{"  - [ ] indented", true},
		{"    - [x] deep indent", true},
		{"not a checkbox", false},
		{"- no bracket", false},
		{"- [] empty brackets", false},
		{"[x] no bullet", false},
		{"# Header", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := IsCheckboxLine(tt.line); got != tt.want {
				t.Errorf("IsCheckboxLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestIsCheckedLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"- [x] checked", true},
		{"- [X] checked uppercase", true},
		{"- [ ] unchecked", false},
		{"not a checkbox", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := IsCheckedLine(tt.line); got != tt.want {
				t.Errorf("IsCheckedLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestToggleCheckbox(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"- [ ] unchecked", "- [x] unchecked"},
		{"- [x] checked", "- [ ] checked"},
		{"- [X] checked upper", "- [ ] checked upper"},
		{"  - [ ] indented", "  - [x] indented"},
		{"* [ ] asterisk", "* [x] asterisk"},
		{"not a checkbox", "not a checkbox"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ToggleCheckbox(tt.input); got != tt.want {
				t.Errorf("ToggleCheckbox(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHasUncheckedTodos(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"has unchecked", "- [ ] task\n- [x] done", true},
		{"all checked", "- [x] done\n- [x] also done", false},
		{"no checkboxes", "just text\nmore text", false},
		{"only unchecked", "- [ ] one\n- [ ] two", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasUncheckedTodos(tt.content); got != tt.want {
				t.Errorf("HasUncheckedTodos() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasCheckedTodos(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"has checked", "- [ ] task\n- [x] done", true},
		{"none checked", "- [ ] task\n- [ ] other", false},
		{"no checkboxes", "just text", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasCheckedTodos(tt.content); got != tt.want {
				t.Errorf("HasCheckedTodos() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasAnyTodos(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"has both", "- [ ] task\n- [x] done", true},
		{"only unchecked", "- [ ] task", true},
		{"only checked", "- [x] done", true},
		{"no checkboxes", "just text", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasAnyTodos(tt.content); got != tt.want {
				t.Errorf("HasAnyTodos() = %v, want %v", got, tt.want)
			}
		})
	}
}
