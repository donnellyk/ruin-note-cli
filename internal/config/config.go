package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"gopkg.in/yaml.v3"
)

type Config struct {
	VaultPath      string `yaml:"vault_path"`
	Versioning     *bool  `yaml:"versioning,omitempty"`
	TagInheritance *bool  `yaml:"tag_inheritance,omitempty"`
	TagFrontmatter *bool  `yaml:"tag_frontmatter,omitempty"`
}

// VersioningEnabled defaults to true. Respects RUIN_VERSIONING env var.
func (c *Config) VersioningEnabled() bool {
	if env := os.Getenv("RUIN_VERSIONING"); env != "" {
		return env != "false" && env != "0"
	}
	if c.Versioning != nil {
		return *c.Versioning
	}
	return true
}

// TagInheritanceEnabled defaults to true. Respects RUIN_TAG_INHERITANCE env var.
func (c *Config) TagInheritanceEnabled() bool {
	if env := os.Getenv("RUIN_TAG_INHERITANCE"); env != "" {
		return env != "false" && env != "0"
	}
	if c.TagInheritance != nil {
		return *c.TagInheritance
	}
	return true
}

// TagFrontmatterEnabled defaults to true. Respects RUIN_TAG_FRONTMATTER env var.
// When false, the vault save path skips writing tags: and inline-tags: to
// frontmatter. inherited-tags: is unaffected.
func (c *Config) TagFrontmatterEnabled() bool {
	if env := os.Getenv("RUIN_TAG_FRONTMATTER"); env != "" {
		return env != "false" && env != "0"
	}
	if c.TagFrontmatter != nil {
		return *c.TagFrontmatter
	}
	return true
}

func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "ruin"), nil
}

func DefaultConfigPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yml"), nil
}

// legacyConfigPath returns the old path ~/.config/ruin only if it exists as a
// regular file (not a directory).
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

// Load reads the configuration using the default path. Environment variables
// RUIN_CONFIG and RUIN_VAULT override file values.
func Load() (*Config, error) {
	return LoadFrom("")
}

// LoadFrom reads the configuration from the specified path. Falls back to
// legacy ~/.config/ruin file if the default path is missing. RUIN_CONFIG and
// RUIN_VAULT env vars override file values.
func LoadFrom(configPath string) (*Config, error) {
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

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Fall back to legacy path if default doesn't exist or parent is a legacy file.
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

	if envVault := os.Getenv("RUIN_VAULT"); envVault != "" {
		cfg.VaultPath = envVault
	}

	return cfg, nil
}

// Save writes the configuration to the default path (respecting RUIN_CONFIG).
func Save(c *Config) error {
	return c.SaveTo("")
}

// SaveTo writes the configuration. If a parent path component is a regular
// file (legacy config), it is removed so the directory can be created.
func (c *Config) SaveTo(configPath string) error {
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
	path := dir
	for {
		info, err := os.Stat(path)
		if err == nil {
			if !info.IsDir() {
				return os.Remove(path)
			}
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

func (c *Config) Validate() error {
	if c.VaultPath == "" {
		return errors.New("vault_path is not configured")
	}

	if len(c.VaultPath) > 0 && c.VaultPath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to expand home directory: %w", err)
		}
		c.VaultPath = filepath.Join(home, c.VaultPath[1:])
	}

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
