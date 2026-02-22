package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCmd_CurrentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config")

	// Isolate config to temp file
	t.Setenv("RUIN_CONFIG", configFile)

	// Change to temp directory for the test
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	jsonOut := false
	cmd := NewInitCmd(&jsonOut)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{})
	err = cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	// Should mention the temp directory
	if !strings.Contains(output, tmpDir) {
		t.Errorf("output = %q, want to contain %q", output, tmpDir)
	}

	// .ruin directory should exist
	ruinDir := filepath.Join(tmpDir, ".ruin")
	if _, err := os.Stat(ruinDir); err != nil {
		t.Errorf(".ruin directory not created: %v", err)
	}

	// tags.yml should exist
	tagsFile := filepath.Join(ruinDir, "tags.yml")
	if _, err := os.Stat(tagsFile); err != nil {
		t.Errorf("tags.yml not created: %v", err)
	}

	// Config should NOT be updated when no path arg provided
	if _, err := os.Stat(configFile); err == nil {
		t.Error("config file should not be created when no path arg provided")
	}
}

func TestInitCmd_WithPath(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config")
	vaultDir := filepath.Join(tmpDir, "my-vault")

	// Isolate config to temp file
	t.Setenv("RUIN_CONFIG", configFile)

	jsonOut := false
	cmd := NewInitCmd(&jsonOut)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{vaultDir})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	// Should mention the vault directory
	if !strings.Contains(output, vaultDir) {
		t.Errorf("output = %q, want to contain %q", output, vaultDir)
	}

	// Vault directory should be created
	if _, err := os.Stat(vaultDir); err != nil {
		t.Errorf("vault directory not created: %v", err)
	}

	// .ruin directory should exist
	ruinDir := filepath.Join(vaultDir, ".ruin")
	if _, err := os.Stat(ruinDir); err != nil {
		t.Errorf(".ruin directory not created: %v", err)
	}

	// Config SHOULD be updated when path arg provided
	configData, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("config file should be created when path arg provided: %v", err)
	}
	if !strings.Contains(string(configData), vaultDir) {
		t.Errorf("config should contain vault path, got: %s", configData)
	}
}

func TestInitCmd_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config")
	vaultDir := filepath.Join(tmpDir, "json-vault")

	// Isolate config to temp file
	t.Setenv("RUIN_CONFIG", configFile)

	jsonOut := true
	cmd := NewInitCmd(&jsonOut)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{vaultDir})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output InitOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if output.Vault != vaultDir {
		t.Errorf("Vault = %q, want %q", output.Vault, vaultDir)
	}

	if len(output.Created) == 0 {
		t.Error("Created should not be empty")
	}
}

func TestInitCmd_Force(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config")
	vaultDir := filepath.Join(tmpDir, "force-vault")

	// Isolate config to temp file
	t.Setenv("RUIN_CONFIG", configFile)

	// First init
	jsonOut := false
	cmd := NewInitCmd(&jsonOut)
	cmd.SetArgs([]string{vaultDir})
	cmd.Execute()

	// Modify tags.yml to verify force overwrites
	tagsFile := filepath.Join(vaultDir, ".ruin", "tags.yml")
	os.WriteFile(tagsFile, []byte("modified: true"), 0644)

	// Second init with --force
	cmd2 := NewInitCmd(&jsonOut)
	cmd2.SetArgs([]string{"--force", vaultDir})
	err := cmd2.Execute()

	if err != nil {
		t.Fatalf("Execute() with --force error = %v", err)
	}

	// tags.yml should be overwritten
	data, _ := os.ReadFile(tagsFile)
	if strings.Contains(string(data), "modified") {
		t.Error("--force should overwrite existing files")
	}
}

func TestInitCmd_ConfigFlag(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	// Isolate config to temp file
	t.Setenv("RUIN_CONFIG", configFile)

	// Change to temp directory for the test
	vaultDir := filepath.Join(tmpDir, "vault")
	os.MkdirAll(vaultDir, 0755)
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(vaultDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	jsonOut := false
	cmd := NewInitCmd(&jsonOut)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// No path arg, but --config flag
	cmd.SetArgs([]string{"--config"})
	err = cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	// Should mention config path
	if !strings.Contains(output, "Config:") {
		t.Errorf("output should mention config, got: %q", output)
	}

	// Config file should be created with vault path
	configData, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("config file should be created with --config flag: %v", err)
	}
	if !strings.Contains(string(configData), vaultDir) {
		t.Errorf("config should contain vault path %q, got: %s", vaultDir, configData)
	}
}

func TestInitCmd_ConfigFlagJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	// Isolate config to temp file
	t.Setenv("RUIN_CONFIG", configFile)

	vaultDir := filepath.Join(tmpDir, "vault")
	os.MkdirAll(vaultDir, 0755)
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(vaultDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	jsonOut := true
	cmd := NewInitCmd(&jsonOut)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--config"})
	err = cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output InitOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if output.Config != configFile {
		t.Errorf("Config = %q, want %q", output.Config, configFile)
	}
}

func TestInitCmd_NoConfigWithoutFlag(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	t.Setenv("RUIN_CONFIG", configFile)

	vaultDir := filepath.Join(tmpDir, "vault")
	os.MkdirAll(vaultDir, 0755)
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(vaultDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	jsonOut := true
	cmd := NewInitCmd(&jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// No path arg, no --config flag
	cmd.SetArgs([]string{})
	err = cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output InitOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Config should be empty when neither path arg nor --config flag
	if output.Config != "" {
		t.Errorf("Config should be empty without --config flag, got %q", output.Config)
	}

	// Config file should not exist
	if _, err := os.Stat(configFile); err == nil {
		t.Error("config file should not be created without --config flag")
	}
}

func TestInitCmd_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config")
	vaultDir := filepath.Join(tmpDir, "idempotent-vault")

	// Isolate config to temp file
	t.Setenv("RUIN_CONFIG", configFile)

	jsonOut := true

	// First init
	cmd := NewInitCmd(&jsonOut)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	cmd.SetArgs([]string{vaultDir})
	cmd.Execute()
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	var output1 InitOutput
	json.Unmarshal(buf.Bytes(), &output1)

	// Second init (without --force)
	cmd2 := NewInitCmd(&jsonOut)
	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	cmd2.SetArgs([]string{vaultDir})
	cmd2.Execute()
	w2.Close()
	os.Stdout = oldStdout

	var buf2 bytes.Buffer
	buf2.ReadFrom(r2)
	var output2 InitOutput
	json.Unmarshal(buf2.Bytes(), &output2)

	// Second run should report files as existed, not created
	if len(output2.Existed) == 0 {
		t.Error("second init should report files as existed")
	}
	if len(output2.Created) > 0 {
		t.Error("second init should not create new files")
	}
}
