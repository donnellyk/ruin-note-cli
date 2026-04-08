package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

func TestQuerySaveCmd(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// Capture stdout
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	cmd.SetArgs([]string{"save", "test-query", "#daily", "--force"})
	err := cmd.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var bufErr bytes.Buffer
	bufErr.ReadFrom(rErr)
	stderr := bufErr.String()

	// Should show match count
	if !strings.Contains(stderr, "matches 2 notes") {
		t.Errorf("stderr = %q, want to contain 'matches 2 notes'", stderr)
	}

	// Should show saved message
	if !strings.Contains(stderr, "Saved query") {
		t.Errorf("stderr = %q, want to contain 'Saved query'", stderr)
	}

	rOut.Close()

	// Verify query was saved
	index, err := vlt.LoadQueries()
	if err != nil {
		t.Fatalf("LoadQueries() error = %v", err)
	}

	found := false
	for _, q := range index.Queries {
		if q.Name == "test-query" && q.Query == "#daily" {
			found = true
			break
		}
	}

	if !found {
		t.Error("query was not saved to queries.yml")
	}
}

func TestQuerySaveCmd_JSON(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := true
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// Capture stdout
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	_, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	cmd.SetArgs([]string{"save", "json-test", "#work", "--force"})
	err := cmd.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(rOut)

	var result struct {
		Name    string `json:"name"`
		Query   string `json:"query"`
		Matches int    `json:"matches"`
	}

	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.Name != "json-test" {
		t.Errorf("name = %q, want %q", result.Name, "json-test")
	}

	if result.Query != "#work" {
		t.Errorf("query = %q, want %q", result.Query, "#work")
	}

	if result.Matches != 2 {
		t.Errorf("matches = %d, want 2", result.Matches)
	}
}

func TestQuerySaveCmd_Update(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// First save
	cmd.SetArgs([]string{"save", "update-test", "#daily", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first save error = %v", err)
	}

	// Update with different query
	cmd.SetArgs([]string{"save", "update-test", "#work", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update save error = %v", err)
	}

	// Verify updated
	index, err := vlt.LoadQueries()
	if err != nil {
		t.Fatalf("LoadQueries() error = %v", err)
	}

	for _, q := range index.Queries {
		if q.Name == "update-test" {
			if q.Query != "#work" {
				t.Errorf("query = %q, want %q after update", q.Query, "#work")
			}
			return
		}
	}

	t.Error("query 'update-test' not found")
}

func TestQueryListCmd(t *testing.T) {
	vlt := setupTestVault(t)

	// Save some queries first
	index := &vault.QueriesIndex{
		Queries: []vault.QueryEntry{
			{Name: "query1", Query: "#daily"},
			{Name: "query2", Query: "#work"},
		},
	}
	if err := vlt.SaveQueries(index); err != nil {
		t.Fatalf("SaveQueries() error = %v", err)
	}

	jsonOut := false
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"list"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "query1: #daily") {
		t.Errorf("output missing query1, got: %q", output)
	}

	if !strings.Contains(output, "query2: #work") {
		t.Errorf("output missing query2, got: %q", output)
	}
}

func TestQueryListCmd_JSON(t *testing.T) {
	vlt := setupTestVault(t)

	// Save a query first
	index := &vault.QueriesIndex{
		Queries: []vault.QueryEntry{
			{Name: "json-list", Query: "#test"},
		},
	}
	if err := vlt.SaveQueries(index); err != nil {
		t.Fatalf("SaveQueries() error = %v", err)
	}

	jsonOut := true
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"list"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var results []struct {
		Name  string `json:"name"`
		Query string `json:"query"`
	}

	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}

	if results[0].Name != "json-list" {
		t.Errorf("name = %q, want %q", results[0].Name, "json-list")
	}
}

func TestQueryListCmd_Empty(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"list"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No saved queries") {
		t.Errorf("output = %q, want to contain 'No saved queries'", output)
	}
}

func TestQueryDeleteCmd(t *testing.T) {
	vlt := setupTestVault(t)

	// Save a query first
	index := &vault.QueriesIndex{
		Queries: []vault.QueryEntry{
			{Name: "to-delete", Query: "#test"},
			{Name: "to-keep", Query: "#other"},
		},
	}
	if err := vlt.SaveQueries(index); err != nil {
		t.Fatalf("SaveQueries() error = %v", err)
	}

	jsonOut := false
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"delete", "to-delete", "--force"})
	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify deleted
	index, err = vlt.LoadQueries()
	if err != nil {
		t.Fatalf("LoadQueries() error = %v", err)
	}

	if len(index.Queries) != 1 {
		t.Errorf("got %d queries, want 1", len(index.Queries))
	}

	if index.Queries[0].Name != "to-keep" {
		t.Errorf("remaining query name = %q, want %q", index.Queries[0].Name, "to-keep")
	}
}

func TestQueryDeleteCmd_NotFound(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"delete", "nonexistent", "--force"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for nonexistent query")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestQueryRunCmd(t *testing.T) {
	vlt := setupTestVault(t)

	// Save a query first
	index := &vault.QueriesIndex{
		Queries: []vault.QueryEntry{
			{Name: "run-test", Query: "#daily"},
		},
	}
	if err := vlt.SaveQueries(index); err != nil {
		t.Fatalf("SaveQueries() error = %v", err)
	}

	jsonOut := false
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"run", "run-test"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should find notes with #daily
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("found %d results, want 2", len(lines))
	}
}

func TestQueryRunCmd_NotFound(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"run", "nonexistent"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for nonexistent query")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestQueryRunCmd_WithFlags(t *testing.T) {
	vlt := setupTestVault(t)

	// Save a query first
	index := &vault.QueriesIndex{
		Queries: []vault.QueryEntry{
			{Name: "flags-test", Query: "#daily"},
		},
	}
	if err := vlt.SaveQueries(index); err != nil {
		t.Fatalf("SaveQueries() error = %v", err)
	}

	jsonOut := false
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"run", "flags-test", "--limit", "1"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("found %d results, want 1 (limited)", len(lines))
	}
}

func TestQuerySaveCmd_InvalidQuery(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"save", "invalid", "", "--force"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for empty query")
	}

	if !strings.Contains(err.Error(), "invalid query") {
		t.Errorf("error = %q, want to contain 'invalid query'", err.Error())
	}
}

func TestQuerySaveCmd_NonInteractive(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewQueryCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// Redirect stderr to test non-interactive detection
	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	cmd.SetArgs([]string{"save", "non-interactive", "#daily"})
	err := cmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	if err == nil {
		t.Error("expected error in non-interactive mode without --force")
	}

	if !strings.Contains(err.Error(), "non-interactive") {
		t.Errorf("error = %q, want to contain 'non-interactive'", err.Error())
	}
}

func TestCountMatches(t *testing.T) {
	vlt := setupTestVault(t)

	matcher, info, _ := parseQuery("#daily", TagScopeAll)
	count, err := countMatches(vlt, matcher, info)

	if err != nil {
		t.Fatalf("countMatches() error = %v", err)
	}

	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}
