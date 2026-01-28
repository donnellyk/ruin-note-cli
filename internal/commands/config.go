package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/kevin/ruin-note-cli/internal/config"
	"github.com/spf13/cobra"
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
  vault_path  - Path to the notes vault directory`,
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
	switch strings.ToLower(key) {
	case "vault_path", "vault-path", "vaultpath":
		return cfg.VaultPath, nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

func setConfigValue(cfg *config.Config, key, value string) error {
	switch strings.ToLower(key) {
	case "vault_path", "vault-path", "vaultpath":
		cfg.VaultPath = value
		return nil
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
}

// ListConfigKeys returns all available config keys (for shell completion).
func ListConfigKeys() []string {
	cfg := &config.Config{}
	t := reflect.TypeOf(*cfg)
	keys := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("yaml")
		if tag != "" && tag != "-" {
			keys = append(keys, tag)
		}
	}
	return keys
}
