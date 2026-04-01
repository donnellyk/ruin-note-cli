package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	VaultPath      string `yaml:"vault_path"`
	Versioning     *bool  `yaml:"versioning,omitempty"`
	TagInheritance *bool  `yaml:"tag_inheritance,omitempty"`
}

// VersioningEnabled returns whether versioning is enabled.
// Defaults to true if not explicitly set. Respects RUIN_VERSIONING env var.
func (c *Config) VersioningEnabled() bool {
	if env := os.Getenv("RUIN_VERSIONING"); env != "" {
		return env != "false" && env != "0"
	}
	if c.Versioning != nil {
		return *c.Versioning
	}
	return true
}

// TagInheritanceEnabled returns whether tag inheritance is enabled.
// Defaults to true if not explicitly set. Respects RUIN_TAG_INHERITANCE env var.
func (c *Config) TagInheritanceEnabled() bool {
	if env := os.Getenv("RUIN_TAG_INHERITANCE"); env != "" {
		return env != "false" && env != "0"
	}
	if c.TagInheritance != nil {
		return *c.TagInheritance
	}
	return true
}

// DefaultConfigDir returns the default config directory path (~/.config/ruin/).
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "ruin"), nil
}

// DefaultConfigPath returns the default config file path (~/.config/ruin/config.yml).
func DefaultConfigPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yml"), nil
}

// legacyConfigPath returns the old config file path (~/.config/ruin) for backwards compatibility.
// Returns the path only if it exists and is a regular file (not a directory).
func legacyConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".config", "ruin")
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return ""
	}
	return path
}

// Load reads the configuration using the default path.
// Environment variables override file values:
//   - RUIN_CONFIG: overrides config file path
//   - RUIN_VAULT: overrides vault_path
func Load() (*Config, error) {
	return LoadFrom("")
}

// LoadFrom reads the configuration from the specified path.
// If configPath is empty, it uses the default path.
// For backwards compatibility, falls back to ~/.config/ruin if it exists as a file.
// Environment variables override file values:
//   - RUIN_CONFIG: overrides config file path
//   - RUIN_VAULT: overrides vault_path
func LoadFrom(configPath string) (*Config, error) {
	// Determine config file path
	usingDefault := configPath == "" && os.Getenv("RUIN_CONFIG") == ""
	if envConfig := os.Getenv("RUIN_CONFIG"); envConfig != "" {
		configPath = envConfig
	}
	if configPath == "" {
		var err error
		configPath, err = DefaultConfigPath()
		if err != nil {
			return nil, err
		}
	}

	cfg := &Config{}

	// Read config file if it exists
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// New default path doesn't exist (or parent is a legacy file) — try legacy path
		if usingDefault {
			if legacy := legacyConfigPath(); legacy != "" {
				data, err = os.ReadFile(legacy)
				if err != nil {
					data = nil
				}
			}
		}
	}

	if data != nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Environment variable overrides
	if envVault := os.Getenv("RUIN_VAULT"); envVault != "" {
		cfg.VaultPath = envVault
	}

	return cfg, nil
}

// Save writes the configuration to the default path.
// Respects RUIN_CONFIG environment variable.
func Save(c *Config) error {
	return c.SaveTo("")
}

// SaveTo writes the configuration to the specified path.
// If configPath is empty, uses RUIN_CONFIG env var or default path.
// If a parent path component is a regular file (legacy config), it is
// removed so the directory structure can be created.
func (c *Config) SaveTo(configPath string) error {
	// Check environment variable first
	if configPath == "" {
		if envConfig := os.Getenv("RUIN_CONFIG"); envConfig != "" {
			configPath = envConfig
		}
	}
	if configPath == "" {
		var err error
		configPath, err = DefaultConfigPath()
		if err != nil {
			return err
		}
	}

	// Ensure parent directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		// MkdirAll fails with ENOTDIR when a parent path component is a
		// regular file (e.g. legacy ~/.config/ruin file blocking directory
		// creation). Remove the blocking file and retry.
		if errors.Is(err, syscall.ENOTDIR) {
			if migrateErr := removeBlockingFile(dir); migrateErr != nil {
				return fmt.Errorf("failed to migrate legacy config: %w", migrateErr)
			}
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}
		} else {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// removeBlockingFile walks up the directory path to find a regular file that
// is blocking directory creation, and removes it.
func removeBlockingFile(dir string) error {
	// Walk up from dir until we find a component that exists as a regular file
	path := dir
	for {
		info, err := os.Stat(path)
		if err == nil {
			if !info.IsDir() {
				return os.Remove(path)
			}
			// It's already a directory — nothing to remove
			return fmt.Errorf("no blocking file found in path: %s", dir)
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return fmt.Errorf("no blocking file found in path: %s", dir)
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.VaultPath == "" {
		return errors.New("vault_path is not configured")
	}

	// Expand ~ in path
	if len(c.VaultPath) > 0 && c.VaultPath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to expand home directory: %w", err)
		}
		c.VaultPath = filepath.Join(home, c.VaultPath[1:])
	}

	// Check if vault path exists
	info, err := os.Stat(c.VaultPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("vault path does not exist: %s", c.VaultPath)
		}
		return fmt.Errorf("failed to access vault path: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("vault path is not a directory: %s", c.VaultPath)
	}

	return nil
}

// ExpandedVaultPath returns the vault path with ~ expanded.
func (c *Config) ExpandedVaultPath() (string, error) {
	path := c.VaultPath
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}
	return path, nil
}
