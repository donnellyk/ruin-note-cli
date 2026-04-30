package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

// writeTestNote creates a markdown file with minimal frontmatter and a tag in
// the body so the doctor scan has actual work to do (rebuilds tags index).
func writeTestNote(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestMaybeRunDoctorOnInit_EmptyVault(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	out, err := maybeRunDoctorOnInit(vlt, false, false, strings.NewReader(""), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil DoctorOutput on empty vault, got %+v", out)
	}
}

func TestMaybeRunDoctorOnInit_ForceSkipsPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	writeTestNote(t, tmpDir, "note1.md", "# Note 1\n\nBody with #tag.\n")

	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	var stderr bytes.Buffer
	out, err := maybeRunDoctorOnInit(vlt, true, false, strings.NewReader(""), &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatalf("expected DoctorOutput when --force runs doctor, got nil")
	}
	if out.Scanned != 1 {
		t.Errorf("Scanned = %d, want 1", out.Scanned)
	}
	if strings.Contains(stderr.String(), "[y/N]") {
		t.Errorf("--force should skip the prompt, but stderr contained it: %s", stderr.String())
	}
}

func TestMaybeRunDoctorOnInit_NonInteractiveErrorsWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()
	writeTestNote(t, tmpDir, "note1.md", "# Note 1\n\n#tag\n")

	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// strings.Reader is not an *os.File, so isTerminal returns false → non-interactive.
	_, err := maybeRunDoctorOnInit(vlt, false, false, strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatalf("expected error in non-interactive mode without --force")
	}
	if !strings.Contains(err.Error(), "non-interactive") {
		t.Errorf("error should mention non-interactive, got %v", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention --force, got %v", err)
	}
}

// TestInitCmd_NoteFolderRequiresForce exercises the cobra command end-to-end:
// init against a folder with existing notes errors when stdin is non-tty and
// --force isn't set. This protects the "non-interactive shells must opt in"
// invariant.
func TestInitCmd_NoteFolderRequiresForce(t *testing.T) {
	tmpDir := t.TempDir()
	writeTestNote(t, tmpDir, "note1.md", "# Note 1\n\n#tag\n")
	writeTestNote(t, tmpDir, "note2.md", "# Note 2\n\nBody #other\n")

	configFile := filepath.Join(tmpDir, "config")
	t.Setenv("RUIN_CONFIG", configFile)

	// Replace os.Stdin with a pipe (not a char device), so isTerminal returns
	// false and init takes the non-interactive branch deterministically.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		r.Close()
	}()

	jsonOut := false
	cmd := NewInitCmd(&jsonOut)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error when running init with existing notes and no --force in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("expected --force hint in error, got %v", err)
	}
}

// TestInitCmd_NoteFolderWithForceRunsDoctor verifies the happy path for the
// migration scenario: a folder of foreign notes + --force builds the indices.
func TestInitCmd_NoteFolderWithForceRunsDoctor(t *testing.T) {
	tmpDir := t.TempDir()
	writeTestNote(t, tmpDir, "note1.md", "# Note 1\n\n#tag\n")
	writeTestNote(t, tmpDir, "note2.md", "# Note 2\n\nBody #other\n")

	configFile := filepath.Join(tmpDir, "config")
	t.Setenv("RUIN_CONFIG", configFile)

	jsonOut := true
	cmd := NewInitCmd(&jsonOut)
	cmd.SetArgs([]string{tmpDir, "--force"})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	if err := cmd.Execute(); err != nil {
		w.Close()
		t.Fatalf("Execute: %v", err)
	}
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if !strings.Contains(buf.String(), `"doctor":`) {
		t.Errorf("expected doctor output embedded in init JSON, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"scanned": 2`) {
		t.Errorf("expected scanned: 2 in doctor output, got: %s", buf.String())
	}

	// Verify the tags index was built.
	vlt := vault.New(tmpDir)
	tags, err := vlt.LoadTags()
	if err != nil {
		t.Fatalf("LoadTags: %v", err)
	}
	if len(tags.Tags) == 0 {
		t.Errorf("expected tags index to be populated after init+doctor, got empty")
	}
}

// TestInitCmd_NoteFolderEmptySkipsDoctor verifies that an empty target folder
// does not trigger the doctor flow at all (no JSON 'doctor' key, no prompt).
func TestInitCmd_NoteFolderEmptySkipsDoctor(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config")
	t.Setenv("RUIN_CONFIG", configFile)

	jsonOut := true
	cmd := NewInitCmd(&jsonOut)
	cmd.SetArgs([]string{tmpDir})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	if err := cmd.Execute(); err != nil {
		w.Close()
		t.Fatalf("Execute: %v", err)
	}
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if strings.Contains(buf.String(), `"doctor":`) {
		t.Errorf("expected no doctor key when vault is empty, got: %s", buf.String())
	}
}
