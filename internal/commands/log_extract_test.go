package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

func newTestLogCmd(t *testing.T) (*bool, func(args ...string) (string, error)) {
	t.Helper()
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	jsonOut := false
	cmd := NewLogCmd(func() *vault.Vault { return vlt }, &jsonOut)

	run := func(args ...string) (string, error) {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs(args)
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		return buf.String(), err
	}

	return &jsonOut, run
}

func TestLogExtract_BasicTags(t *testing.T) {
	_, run := newTestLogCmd(t)

	out, err := run("extract", "#global1 #global2\n\nSome text #inline here.")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out, "global:") {
		t.Error("output should contain 'global:' section")
	}
	if !strings.Contains(out, "#global1") {
		t.Errorf("output should contain #global1, got: %s", out)
	}
	if !strings.Contains(out, "#global2") {
		t.Errorf("output should contain #global2, got: %s", out)
	}
	if !strings.Contains(out, "inline:") {
		t.Error("output should contain 'inline:' section")
	}
	if !strings.Contains(out, "#inline") {
		t.Errorf("output should contain #inline, got: %s", out)
	}
}

func TestLogExtract_JSONOutput(t *testing.T) {
	jsonOut, run := newTestLogCmd(t)
	*jsonOut = true
	defer func() { *jsonOut = false }()

	out, err := run("extract", "#global\n\nText #inline here.")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var result extractOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out)
	}

	if len(result.Global) != 1 || result.Global[0] != "#global" {
		t.Errorf("global = %v, want [#global]", result.Global)
	}
	if len(result.Inline) != 1 || result.Inline[0] != "#inline" {
		t.Errorf("inline = %v, want [#inline]", result.Inline)
	}
}

func TestLogExtract_EmptyTagsJSON(t *testing.T) {
	jsonOut, run := newTestLogCmd(t)
	*jsonOut = true
	defer func() { *jsonOut = false }()

	out, err := run("extract", "No tags here at all.")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var result extractOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out)
	}

	// Should be empty arrays, not null
	if result.Global == nil {
		t.Error("global should be [] not null")
	}
	if result.Inline == nil {
		t.Error("inline should be [] not null")
	}
	if len(result.Global) != 0 || len(result.Inline) != 0 {
		t.Errorf("expected empty arrays, got global=%v inline=%v", result.Global, result.Inline)
	}
}

func TestLogExtract_NoContent(t *testing.T) {
	_, run := newTestLogCmd(t)

	_, err := run("extract")
	if err == nil {
		t.Error("expected error for no content")
	}
}

func TestLogExtract_WithTitle(t *testing.T) {
	_, run := newTestLogCmd(t)

	// Title shouldn't change classification in this case, but verify flag is accepted
	out, err := run("extract", "--title", "My Note", "#tag1\n\nContent #tag2")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out, "#tag1") {
		t.Errorf("output should contain #tag1, got: %s", out)
	}
	if !strings.Contains(out, "#tag2") {
		t.Errorf("output should contain #tag2, got: %s", out)
	}
}

func TestLogExtract_EmptyTagsText(t *testing.T) {
	_, run := newTestLogCmd(t)

	out, err := run("extract", "Just plain text, no tags.")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out, "global:") {
		t.Error("output should contain 'global:' even with no tags")
	}
	if !strings.Contains(out, "inline:") {
		t.Error("output should contain 'inline:' even with no tags")
	}
}
