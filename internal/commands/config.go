package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/spf13/cobra"
	"kvnd/ruin-note-cli/internal/config"
)

// NewConfigCmd creates the config command.
func NewConfigCmd(jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config [key] [value]",
		Short: "View or modify configuration",
		Long: `View or modify the ruin configuration.

With no arguments, displays all configuration values.
With one argument, displays the value for that key.
With two arguments, sets the key to the value.

Available keys:
  vault_path       - Path to the notes vault directory
  versioning       - Enable/disable git auto-versioning (true/false)
  tag_inheritance  - Enable/disable inherited tags from parent notes (true/false)`,
		Example: `  # Show all config
  ruin config

  # Show specific value
  ruin config vault_path

  # Set a value
  ruin config vault_path ~/notes`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				// Config might not exist, create empty one
				cfg = &config.Config{}
			}

			switch len(args) {
			case 0:
				// Show all config
				return showAllConfig(cfg, *jsonOutput)
			case 1:
				// Show specific key
				return showConfigKey(cfg, args[0], *jsonOutput)
			case 2:
				// Set key=value
				return setConfigKey(cfg, args[0], args[1], *jsonOutput)
			}

			return nil
		},
	}

	return cmd
}

func showAllConfig(cfg *config.Config, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cfg)
	}

	// Human-readable output
	fmt.Printf("vault_path: %s\n", cfg.VaultPath)
	return nil
}

func showConfigKey(cfg *config.Config, key string, jsonOut bool) error {
	value, err := getConfigValue(cfg, key)
	if err != nil {
		return err
	}

	if jsonOut {
		output := map[string]string{key: value}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	fmt.Println(value)
	return nil
}

func setConfigKey(cfg *config.Config, key, value string, jsonOut bool) error {
	if err := setConfigValue(cfg, key, value); err != nil {
		return err
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if jsonOut {
		output := map[string]string{key: value, "status": "saved"}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

func getConfigValue(cfg *config.Config, key string) (string, error) {
	switch key {
	case "vault_path":
		return cfg.VaultPath, nil
	case "versioning":
		return fmt.Sprintf("%t", cfg.VersioningEnabled()), nil
	case "tag_inheritance":
		return fmt.Sprintf("%t", cfg.TagInheritanceEnabled()), nil
	default:
		return "", fmt.Errorf("unknown config key: %s (available: vault_path, versioning, tag_inheritance)", key)
	}
}

func setConfigValue(cfg *config.Config, key, value string) error {
	switch key {
	case "vault_path":
		cfg.VaultPath = value
		return nil
	case "versioning":
		b, err := parseBoolValue(value)
		if err != nil {
			return fmt.Errorf("versioning: %w", err)
		}
		cfg.Versioning = &b
		return nil
	case "tag_inheritance":
		b, err := parseBoolValue(value)
		if err != nil {
			return fmt.Errorf("tag_inheritance: %w", err)
		}
		cfg.TagInheritance = &b
		return nil
	default:
		return fmt.Errorf("unknown config key: %s (available: vault_path, versioning, tag_inheritance)", key)
	}
}

func parseBoolValue(s string) (bool, error) {
	switch s {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("expected true or false, got %q", s)
	}
}

// ListConfigKeys returns all available config keys (for shell completion).
func ListConfigKeys() []string {
	t := reflect.TypeFor[config.Config]()
	keys := make([]string, 0, t.NumField())
	for field := range t.Fields() {
		tag := field.Tag.Get("yaml")
		if tag != "" && tag != "-" {
			keys = append(keys, tag)
		}
	}
	return keys
}
