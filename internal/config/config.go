package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	VaultPath string `yaml:"vault_path"`
}

// DefaultConfigPath returns the default config file path (~/.config/ruin).
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "ruin"), nil
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
// Environment variables override file values:
//   - RUIN_CONFIG: overrides config file path
//   - RUIN_VAULT: overrides vault_path
func LoadFrom(configPath string) (*Config, error) {
	// Determine config file path
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
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file doesn't exist, use defaults
	} else {
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
		return fmt.Errorf("failed to create config directory: %w", err)
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
