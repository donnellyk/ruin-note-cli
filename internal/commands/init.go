package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"kvnd/ruin-note-cli/internal/config"
	"kvnd/ruin-note-cli/internal/vault"
	"kvnd/ruin-note-cli/internal/versioning"
)

// InitOutput represents the JSON output for the init command.
type InitOutput struct {
	Vault   string   `json:"vault"`
	Created []string `json:"created,omitempty"`
	Existed []string `json:"existed,omitempty"`
	Git     bool     `json:"git"`
}

// NewInitCmd creates the init command.
func NewInitCmd(jsonOutput *bool) *cobra.Command {
	var force bool
	var noGit bool

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a notes vault",
		Long: `Initialize a notes vault at the specified path.

Creates the .ruin/ directory with metadata files (tags.yml, queries.yml).
If no path is provided, initializes in the current directory.

If a path is provided, also updates the config file to set this as the default vault.`,
		Example: `  # Initialize in current directory
  ruin init

  # Initialize at specific path
  ruin init ~/notes

  # Re-initialize (overwrite existing metadata)
  ruin init --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine vault path
			var vaultPath string
			if len(args) > 0 {
				vaultPath = args[0]
			} else {
				var err error
				vaultPath, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get current directory: %w", err)
				}
			}

			// Expand ~ and make absolute
			if vaultPath[:1] == "~" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to expand home directory: %w", err)
				}
				vaultPath = filepath.Join(home, vaultPath[1:])
			}

			absPath, err := filepath.Abs(vaultPath)
			if err != nil {
				return fmt.Errorf("failed to resolve path: %w", err)
			}
			vaultPath = absPath

			// Create vault directory if it doesn't exist
			if err := os.MkdirAll(vaultPath, 0755); err != nil {
				return fmt.Errorf("failed to create vault directory: %w", err)
			}

			// Initialize vault
			vlt := vault.New(vaultPath)
			result, err := vlt.Initialize(force)
			if err != nil {
				return fmt.Errorf("failed to initialize vault: %w", err)
			}

			// If path was explicitly provided, update config
			if len(args) > 0 {
				cfg, err := config.Load()
				if err != nil {
					// Config might not exist yet, create new one
					cfg = &config.Config{}
				}
				cfg.VaultPath = vaultPath
				if err := config.Save(cfg); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to update config: %v\n", err)
				}
			}

			// Set up git versioning
			gitInitialized := false
			if !noGit && versioning.IsAvailable() {
				g := versioning.New(vaultPath)
				created, err := g.Init()
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: git init failed: %v\n", err)
				} else {
					gitInitialized = true
					if created {
						fmt.Fprintf(os.Stderr, "Initialized git repository\n")
					}
				}
			}

			// Output
			if *jsonOutput {
				output := InitOutput{
					Vault:   vaultPath,
					Created: result.Created,
					Existed: result.Existed,
					Git:     gitInitialized,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Printf("Initialized vault at %s\n", vaultPath)
			if len(result.Created) > 0 {
				fmt.Printf("  Created: %v\n", result.Created)
			}
			if len(result.Existed) > 0 {
				fmt.Printf("  Existed: %v\n", result.Existed)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing metadata files")
	cmd.Flags().BoolVar(&noGit, "no-git", false, "skip git repository initialization")

	return cmd
}
