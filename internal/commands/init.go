package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/config"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/donnellyk/ruin-note-cli/internal/versioning"
	"github.com/spf13/cobra"
)

type InitOutput struct {
	Vault   string        `json:"vault"`
	Created []string      `json:"created,omitempty"`
	Existed []string      `json:"existed,omitempty"`
	Git     bool          `json:"git"`
	Config  string        `json:"config,omitempty"`
	Doctor  *DoctorOutput `json:"doctor,omitempty"`
}

func NewInitCmd(jsonOutput *bool) *cobra.Command {
	var force bool
	var noGit bool
	var setupConfig bool

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a notes vault",
		Long: `Initialize a notes vault at the specified path.

Creates the .ruin/ directory with metadata files (tags.yml, queries.yml).
If no path is provided, initializes in the current directory.

If a path is provided, also updates the config file to set this as the default vault.
Use --config to create the ~/.config/ruin/ directory and config.yml even when no path is given.

If the target directory already contains markdown notes (e.g., when migrating from
Obsidian), init offers to run doctor to build the tags and titles indices. This
may rewrite frontmatter — see the prompt for details. --force skips the prompt
and always runs doctor.`,
		Example: `  # Initialize in current directory
  ruin init

  # Initialize at specific path
  ruin init ~/notes

  # Initialize and set up config directory
  ruin init --config

  # Re-initialize and rebuild indices without prompting
  ruin init --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			if err := os.MkdirAll(vaultPath, 0755); err != nil {
				return fmt.Errorf("failed to create vault directory: %w", err)
			}

			vlt := vault.New(vaultPath)
			result, err := vlt.Initialize(force)
			if err != nil {
				return fmt.Errorf("failed to initialize vault: %w", err)
			}

			var configPath string
			if len(args) > 0 || setupConfig {
				cfg, err := config.Load()
				if err != nil {
					cfg = &config.Config{}
				}
				cfg.VaultPath = vaultPath
				if err := config.Save(cfg); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to update config: %v\n", err)
				} else if envCfg := os.Getenv("RUIN_CONFIG"); envCfg != "" {
					configPath = envCfg
				} else {
					configPath, _ = config.DefaultConfigPath()
				}
			}

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

			doctorOutput, err := maybeRunDoctorOnInit(vlt, force, *jsonOutput, os.Stdin, os.Stderr)
			if err != nil {
				return err
			}

			if *jsonOutput {
				output := InitOutput{
					Vault:   vaultPath,
					Created: result.Created,
					Existed: result.Existed,
					Git:     gitInitialized,
					Config:  configPath,
					Doctor:  doctorOutput,
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
			if configPath != "" {
				fmt.Printf("  Config: %s\n", configPath)
			}
			if doctorOutput != nil {
				if err := doctorPrintOutput(doctorOutput, "", false); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing metadata files and skip the doctor confirmation prompt when notes already exist")
	cmd.Flags().BoolVar(&noGit, "no-git", false, "skip git repository initialization")
	cmd.Flags().BoolVar(&setupConfig, "config", false, "create ~/.config/ruin/ directory and config.yml")

	return cmd
}

// maybeRunDoctorOnInit decides whether to run the doctor full-scan when init
// targets a folder that already contains notes. With --force or after an
// interactive y-prompt, it runs and returns the output. Returns (nil, nil)
// when the user declines or there are no existing notes. Errors when the
// process is non-interactive without --force, since silently rewriting an
// existing vault's frontmatter on first run would be a hostile default.
func maybeRunDoctorOnInit(vlt *vault.Vault, force, jsonOutput bool, stdin io.Reader, stderr io.Writer) (*DoctorOutput, error) {
	notePaths, err := vlt.ListNotes()
	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}
	if len(notePaths) == 0 {
		return nil, nil
	}

	if !force {
		stdinFile, isFile := stdin.(*os.File)
		interactive := isFile && isTerminal(stdinFile)
		if !interactive {
			return nil, fmt.Errorf("found %d existing notes; non-interactive init requires --force to rebuild indices (this may rewrite frontmatter)", len(notePaths))
		}

		fmt.Fprintf(stderr, "Found %d existing notes. Build indices now?\n", len(notePaths))
		fmt.Fprintln(stderr, "This may rewrite frontmatter:")
		fmt.Fprintln(stderr, "  - Add a uuid to notes that don't have one")
		fmt.Fprintln(stderr, "  - Normalize tags (a leading # is added if missing)")
		fmt.Fprintln(stderr, "  - Rebuild the inline-tags and dates indices from note bodies")
		fmt.Fprintln(stderr, "Other frontmatter fields, key order, and comments are preserved.")
		fmt.Fprint(stderr, "[y/N] ")

		reader := bufio.NewReader(stdin)
		response, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(stderr, "Skipped. Run `ruin doctor` later to build indices.")
			return nil, nil
		}
	}

	output, err := RunDoctorFullScan(vlt, false)
	if err != nil {
		return nil, fmt.Errorf("doctor: %w", err)
	}
	return output, nil
}
