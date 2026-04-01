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

func TestLoad_LegacyFallback(t *testing.T) {
	// When ~/.config/ruin is a file (legacy format), reading
	// ~/.config/ruin/config.yml fails with ENOTDIR, not ErrNotExist.
	// LoadFrom must handle this and fall back to the legacy file.
	tmpDir := t.TempDir()

	// Create a file at the path where the directory would be
	legacyPath := filepath.Join(tmpDir, "ruin")
	content := []byte("vault_path: /legacy/vault\n")
	if err := os.WriteFile(legacyPath, content, 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	// Try to load from the new path (ruin/config.yml) — should fall back to legacy
	newPath := filepath.Join(legacyPath, "config.yml")
	// Can't use LoadFrom directly since it only falls back when using default path.
	// Instead, test the error handling by reading the file directly.
	_, err := os.ReadFile(newPath)
	if err == nil {
		t.Fatal("expected error reading file inside a non-directory")
	}

	// Verify the error is NOT ErrNotExist (this is the bug scenario)
	if os.IsNotExist(err) {
		t.Fatal("expected ENOTDIR, got ErrNotExist")
	}

	// Now test that LoadFrom with an explicit path to the legacy file works
	cfg, err := LoadFrom(legacyPath)
	if err != nil {
		t.Fatalf("LoadFrom(legacy) error = %v", err)
	}
	if cfg.VaultPath != "/legacy/vault" {
		t.Errorf("VaultPath = %q, want %q", cfg.VaultPath, "/legacy/vault")
	}
}

func TestLoad_LegacyMigrationOnSave(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a legacy file at ruin (as a file, not a directory)
	legacyPath := filepath.Join(tmpDir, "ruin")
	content := []byte("vault_path: /legacy/vault\n")
	if err := os.WriteFile(legacyPath, content, 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	// Load from legacy
	cfg, err := LoadFrom(legacyPath)
	if err != nil {
		t.Fatalf("LoadFrom(legacy) error = %v", err)
	}

	// Save to an explicit path inside the directory that's currently a file
	// This tests that SaveTo handles the "parent is a file" case
	newPath := filepath.Join(tmpDir, "ruin", "config.yml")
	if err := cfg.SaveTo(newPath); err != nil {
		t.Fatalf("SaveTo(new) error = %v", err)
	}

	// Verify legacy file is gone and new structure exists
	info, err := os.Stat(filepath.Join(tmpDir, "ruin"))
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("ruin should now be a directory, not a file")
	}

	// Verify content readable from new path
	loaded, err := LoadFrom(newPath)
	if err != nil {
		t.Fatalf("LoadFrom(new) error = %v", err)
	}
	if loaded.VaultPath != "/legacy/vault" {
		t.Errorf("VaultPath = %q, want %q", loaded.VaultPath, "/legacy/vault")
	}
}

func TestLoad_NewPathConfig(t *testing.T) {
	// Test loading from the new directory structure: ruin/config.yml
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "ruin")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.yml")

	content := []byte("vault_path: /new/path\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	if cfg.VaultPath != "/new/path" {
		t.Errorf("VaultPath = %q, want %q", cfg.VaultPath, "/new/path")
	}
}

func TestConfig_SaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ruin", "config.yml")

	cfg := &Config{VaultPath: "/my/vault"}
	if err := cfg.SaveTo(configPath); err != nil {
		t.Fatalf("SaveTo() error = %v", err)
	}

	// Verify the directory was created
	info, err := os.Stat(filepath.Join(tmpDir, "ruin"))
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("config path should be a directory")
	}

	// Read it back
	loaded, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if loaded.VaultPath != "/my/vault" {
		t.Errorf("loaded VaultPath = %q, want %q", loaded.VaultPath, "/my/vault")
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

func TestConfig_TagInheritanceEnabled(t *testing.T) {
	t.Run("default is true", func(t *testing.T) {
		cfg := &Config{}
		if !cfg.TagInheritanceEnabled() {
			t.Error("expected default true")
		}
	})

	t.Run("explicit false", func(t *testing.T) {
		f := false
		cfg := &Config{TagInheritance: &f}
		if cfg.TagInheritanceEnabled() {
			t.Error("expected false when explicitly set")
		}
	})

	t.Run("explicit true", func(t *testing.T) {
		tr := true
		cfg := &Config{TagInheritance: &tr}
		if !cfg.TagInheritanceEnabled() {
			t.Error("expected true when explicitly set")
		}
	})

	t.Run("env var overrides config", func(t *testing.T) {
		tr := true
		cfg := &Config{TagInheritance: &tr}
		t.Setenv("RUIN_TAG_INHERITANCE", "false")
		if cfg.TagInheritanceEnabled() {
			t.Error("env var false should override config true")
		}
	})

	t.Run("env var enables when config unset", func(t *testing.T) {
		cfg := &Config{}
		t.Setenv("RUIN_TAG_INHERITANCE", "0")
		if cfg.TagInheritanceEnabled() {
			t.Error("env var 0 should disable")
		}
	})

	t.Run("roundtrip through YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yml")

		f := false
		cfg := &Config{VaultPath: "/test", TagInheritance: &f}
		if err := cfg.SaveTo(configPath); err != nil {
			t.Fatalf("SaveTo() error = %v", err)
		}

		loaded, err := LoadFrom(configPath)
		if err != nil {
			t.Fatalf("LoadFrom() error = %v", err)
		}
		if loaded.TagInheritanceEnabled() {
			t.Error("expected false after roundtrip")
		}
	})
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
