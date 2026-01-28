package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultsWhenNoFile(t *testing.T) {
	// Use a non-existent config path
	cfg, err := LoadFrom("/nonexistent/path/config")
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	if cfg.VaultPath != "" {
		t.Errorf("expected empty VaultPath, got %q", cfg.VaultPath)
	}
}

func TestLoad_ReadsConfigFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	content := []byte("vault_path: /my/notes\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	if cfg.VaultPath != "/my/notes" {
		t.Errorf("VaultPath = %q, want %q", cfg.VaultPath, "/my/notes")
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	content := []byte("vault_path: /file/path\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Set env var
	t.Setenv("RUIN_VAULT", "/env/path")

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	if cfg.VaultPath != "/env/path" {
		t.Errorf("VaultPath = %q, want %q (env should override file)", cfg.VaultPath, "/env/path")
	}
}

func TestConfig_Save(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config")

	cfg := &Config{VaultPath: "/my/vault"}
	if err := cfg.SaveTo(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read it back
	loaded, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	if loaded.VaultPath != cfg.VaultPath {
		t.Errorf("loaded VaultPath = %q, want %q", loaded.VaultPath, cfg.VaultPath)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		vaultPath string
		setup     func(t *testing.T) string // returns actual path to use
		wantErr   bool
	}{
		{
			name:      "empty vault path",
			vaultPath: "",
			wantErr:   true,
		},
		{
			name:      "nonexistent path",
			vaultPath: "/nonexistent/path/12345",
			wantErr:   true,
		},
		{
			name: "valid directory",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vaultPath := tt.vaultPath
			if tt.setup != nil {
				vaultPath = tt.setup(t)
			}

			cfg := &Config{VaultPath: vaultPath}
			err := cfg.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_ExpandedVaultPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name      string
		vaultPath string
		want      string
	}{
		{
			name:      "absolute path unchanged",
			vaultPath: "/absolute/path",
			want:      "/absolute/path",
		},
		{
			name:      "tilde expanded",
			vaultPath: "~/notes",
			want:      filepath.Join(home, "notes"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{VaultPath: tt.vaultPath}
			got, err := cfg.ExpandedVaultPath()
			if err != nil {
				t.Fatalf("ExpandedVaultPath() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ExpandedVaultPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
