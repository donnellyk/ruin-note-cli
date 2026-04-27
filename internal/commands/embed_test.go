package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

func TestParseEmbedEvalInput(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantType  string
		wantQuery string
		wantErr   bool
	}{
		{
			name:      "full delimiters",
			input:     "![[search: #daily]]",
			wantType:  "search",
			wantQuery: "#daily",
		},
		{
			name:      "full delimiters with options",
			input:     "![[search: #daily | limit=5]]",
			wantType:  "search",
			wantQuery: "#daily",
		},
		{
			name:      "bare form (no delimiters)",
			input:     "search: #daily",
			wantType:  "search",
			wantQuery: "#daily",
		},
		{
			name:      "bare with options",
			input:     "pick: #followup | format=flat",
			wantType:  "pick",
			wantQuery: "#followup",
		},
		{
			name:      "leading and trailing whitespace stripped",
			input:     "   ![[query: my-query]]\n",
			wantType:  "query",
			wantQuery: "my-query",
		},
		{name: "empty input", input: "", wantErr: true},
		{name: "whitespace only", input: "   \t\n", wantErr: true},
		{name: "unknown type", input: "![[foo: bar]]", wantErr: true},
		{name: "missing colon", input: "search #daily", wantErr: true},
		{name: "no inner content", input: "![[search:]]", wantErr: true},
		{name: "bare with stray ]] rejected", input: "search: foo]]", wantErr: true},
		{name: "bare with stray ![[ rejected", input: "search: ![[nested", wantErr: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := parseEmbedEvalInput(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseEmbedEvalInput(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if ref.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", ref.Type, tt.wantType)
			}
			if ref.Query != tt.wantQuery {
				t.Errorf("Query = %q, want %q", ref.Query, tt.wantQuery)
			}
		})
	}
}

// setupEmbedEvalVault creates a vault with notes covering all four embed types.
func setupEmbedEvalVault(t *testing.T) *vault.Vault {
	t.Helper()
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	type fixture struct {
		uuid, filename, title, content string
	}
	notes := []fixture{
		{
			uuid:     "uuid-daily-1",
			filename: "Daily-1.md",
			title:    "Daily 1",
			content: `---
uuid: uuid-daily-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#daily"
inline-tags:
  - "#followup"
---
# Daily 1
#daily

Catch up with Alice. #followup
Buy coffee.`,
		},
		{
			uuid:     "uuid-daily-2",
			filename: "Daily-2.md",
			title:    "Daily 2",
			content: `---
uuid: uuid-daily-2
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
tags:
  - "#daily"
inline-tags:
  - "#followup"
---
# Daily 2
#daily

Send report.  #followup`,
		},
		{
			uuid:     "uuid-hub",
			filename: "Hub.md",
			title:    "Hub",
			content: `---
uuid: uuid-hub
created: "2025-01-05T10:00:00-05:00"
updated: "2025-01-05T10:00:00-05:00"
tags:
  - "#project"
---
# Hub

Hub-level notes.`,
		},
	}

	entries := make(map[string]vault.TitleEntry, len(notes))
	for _, n := range notes {
		path := filepath.Join(tmpDir, n.filename)
		if err := os.WriteFile(path, []byte(n.content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", n.filename, err)
		}
		entries[n.uuid] = vault.TitleEntry{Title: n.title, Path: path}
	}

	if err := vlt.RebuildTitlesIndex(entries); err != nil {
		t.Fatalf("failed to build titles index: %v", err)
	}

	if err := vlt.SaveQueries(&vault.QueriesIndex{
		Queries: []vault.QueryEntry{{Name: "all-daily", Query: "#daily"}},
	}); err != nil {
		t.Fatalf("failed to save queries: %v", err)
	}

	return vlt
}

func runEmbedEval(t *testing.T, vlt *vault.Vault, embedStr string, jsonMode bool) string {
	t.Helper()
	jsonOut := jsonMode
	cmd := NewEmbedCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	cmd.SetArgs([]string{"eval", embedStr})
	err := cmd.Execute()

	w.Close()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestEmbedEval_SearchJSON(t *testing.T) {
	vlt := setupEmbedEvalVault(t)
	out := runEmbedEval(t, vlt, "![[search: #daily | limit=5]]", true)

	var env embedEvalEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("failed to parse JSON: %v\n%s", err, out)
	}
	if env.Type != "search" {
		t.Errorf("Type = %q, want %q", env.Type, "search")
	}
	if env.Query != "#daily" {
		t.Errorf("Query = %q, want %q", env.Query, "#daily")
	}
	if got := env.Options["limit"]; got != "5" {
		t.Errorf("Options[limit] = %q, want %q", got, "5")
	}

	// Re-decode results to verify shape.
	rawResults, err := json.Marshal(env.Results)
	if err != nil {
		t.Fatalf("re-marshal results: %v", err)
	}
	var results []SearchResult
	if err := json.Unmarshal(rawResults, &results); err != nil {
		t.Fatalf("failed to parse results: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestEmbedEval_SearchPlainText(t *testing.T) {
	vlt := setupEmbedEvalVault(t)
	out := runEmbedEval(t, vlt, "![[search: #daily | format=list]]", false)
	if !strings.Contains(out, "[[Daily 1]]") {
		t.Errorf("expected [[Daily 1]] in list output, got %q", out)
	}
	if !strings.Contains(out, "[[Daily 2]]") {
		t.Errorf("expected [[Daily 2]] in list output, got %q", out)
	}
}

func TestEmbedEval_PickJSON(t *testing.T) {
	vlt := setupEmbedEvalVault(t)
	out := runEmbedEval(t, vlt, "![[pick: #followup]]", true)

	var env embedEvalEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("failed to parse JSON: %v\n%s", err, out)
	}
	if env.Type != "pick" {
		t.Errorf("Type = %q, want %q", env.Type, "pick")
	}

	rawResults, _ := json.Marshal(env.Results)
	var results []PickResult
	if err := json.Unmarshal(rawResults, &results); err != nil {
		t.Fatalf("failed to parse results: %v", err)
	}
	if len(results) == 0 {
		t.Errorf("expected at least one pick result")
	}
}

func TestEmbedEval_QueryJSON(t *testing.T) {
	vlt := setupEmbedEvalVault(t)
	out := runEmbedEval(t, vlt, "![[query: all-daily]]", true)

	var env embedEvalEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("failed to parse JSON: %v\n%s", err, out)
	}
	if env.Type != "query" {
		t.Errorf("Type = %q, want %q", env.Type, "query")
	}
	if env.Query != "all-daily" {
		t.Errorf("Query = %q, want %q", env.Query, "all-daily")
	}
}

func TestEmbedEval_ComposeJSON(t *testing.T) {
	vlt := setupEmbedEvalVault(t)
	out := runEmbedEval(t, vlt, "![[compose: Hub]]", true)

	var env embedEvalEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("failed to parse JSON: %v\n%s", err, out)
	}
	if env.Type != "compose" {
		t.Errorf("Type = %q, want %q", env.Type, "compose")
	}

	rawResults, _ := json.Marshal(env.Results)
	var result embedComposeResult
	if err := json.Unmarshal(rawResults, &result); err != nil {
		t.Fatalf("failed to parse compose result: %v", err)
	}
	if !strings.Contains(result.ExpandedMarkdown, "Hub-level notes.") {
		t.Errorf("expected expanded_markdown to contain Hub content, got %q", result.ExpandedMarkdown)
	}
	if len(result.SourceMap) == 0 {
		t.Errorf("expected source_map for compose, got empty")
	}
}

func TestEmbedEval_BareInputAccepted(t *testing.T) {
	vlt := setupEmbedEvalVault(t)
	withDelim := runEmbedEval(t, vlt, "![[search: #daily | limit=5]]", true)
	bare := runEmbedEval(t, vlt, "search: #daily | limit=5", true)
	if withDelim != bare {
		t.Errorf("delimiter and bare forms produced different output\n--with delim--\n%s\n--bare--\n%s", withDelim, bare)
	}
}

func TestEmbedEval_FormatIgnoredInJSONMode(t *testing.T) {
	// format=list shapes plain-text rendering. In JSON mode, results must keep
	// the SearchResult shape and not be coerced to list lines.
	vlt := setupEmbedEvalVault(t)
	out := runEmbedEval(t, vlt, "![[search: #daily | format=list]]", true)

	var env embedEvalEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("failed to parse JSON: %v\n%s", err, out)
	}
	rawResults, _ := json.Marshal(env.Results)
	var results []SearchResult
	if err := json.Unmarshal(rawResults, &results); err != nil {
		t.Fatalf("results should decode as []SearchResult regardless of format=, got %v", err)
	}
	if len(results) == 0 {
		t.Errorf("expected results")
	}
	for _, r := range results {
		if r.UUID == "" {
			t.Errorf("expected UUID on every search result, got %+v", r)
		}
	}
}

func TestEmbedEval_HardErrors(t *testing.T) {
	vlt := setupEmbedEvalVault(t)

	cases := []struct {
		name        string
		args        []string
		wantInError string
	}{
		{
			name:        "unknown embed type",
			args:        []string{"eval", "![[foo: bar]]"},
			wantInError: "invalid embed",
		},
		{
			name:        "empty input",
			args:        []string{"eval", ""},
			wantInError: "empty",
		},
		{
			name:        "compose: note missing",
			args:        []string{"eval", "![[compose: NoSuchNote]]"},
			wantInError: "compose",
		},
		{
			name:        "query: name missing",
			args:        []string{"eval", "![[query: nonexistent]]"},
			wantInError: "not found",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			jsonOut := false
			cmd := NewEmbedCmd(func() *vault.Vault { return vlt }, &jsonOut)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantInError) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantInError)
			}
		})
	}
}

func TestEmbedEval_PickBetweenTolerantOfWhitespace(t *testing.T) {
	// `@between:X, Y` with a space after the comma should parse as a single
	// range, not split into two malformed tokens by strings.Fields. The echoed
	// `query` field preserves the original, so compare results only.
	vlt := setupEmbedEvalVault(t)

	decode := func(s string) []PickResult {
		t.Helper()
		var env embedEvalEnvelope
		if err := json.Unmarshal([]byte(s), &env); err != nil {
			t.Fatalf("decode envelope: %v", err)
		}
		raw, _ := json.Marshal(env.Results)
		var results []PickResult
		if err := json.Unmarshal(raw, &results); err != nil {
			t.Fatalf("decode results: %v", err)
		}
		return results
	}

	withSpace := decode(runEmbedEval(t, vlt, "![[pick: @between:2025-01-01, 2025-12-31]]", true))
	withoutSpace := decode(runEmbedEval(t, vlt, "![[pick: @between:2025-01-01,2025-12-31]]", true))
	if len(withSpace) != len(withoutSpace) {
		t.Errorf("space after comma should not change result count: with=%d without=%d", len(withSpace), len(withoutSpace))
	}
}

func TestEmbedEval_RelativeDateOption(t *testing.T) {
	// Verify created:today-7 in a search embed flows through to the date filter.
	vlt := setupEmbedEvalVault(t)
	// Should not error even if no results match.
	out := runEmbedEval(t, vlt, "![[search: created:today-3650]]", true)
	var env embedEvalEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("failed to parse JSON: %v\n%s", err, out)
	}
	if env.Type != "search" {
		t.Errorf("Type = %q, want %q", env.Type, "search")
	}
}
