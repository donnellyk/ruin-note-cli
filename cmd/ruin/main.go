package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"kvnd/ruin-note-cli/internal/commands"
	"kvnd/ruin-note-cli/internal/config"
	"kvnd/ruin-note-cli/internal/vault"
	"kvnd/ruin-note-cli/internal/versioning"
)

var (
	version = "dev"

	// Global flags
	cfgFile   string
	vaultPath string
	jsonOut   bool
	noColor   bool

	// Loaded at runtime
	cfg *config.Config
	vlt *vault.Vault
)

func main() {
	sanitizeLeadingDashArgs()
	if err := rootCmd.Execute(); err != nil {
		// Determine appropriate exit code based on error type
		switch {
		case err == commands.ErrUserAborted:
			os.Exit(exitUserAborted)
		default:
			// Print the error to stderr
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(exitError)
		}
	}
}

var rootCmd = &cobra.Command{
	Use:     "ruin",
	Short:   "A Zettelkasten-inspired note-taking CLI",
	Version: version,
	Long: `Ruin is a CLI tool for managing markdown notes with frontmatter metadata,
tags, and powerful search capabilities.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == "init" || cmd.Name() == "dev" || cmd.Name() == "seed" {
			return nil
		}

		// Load configuration
		var err error
		cfg, err = config.LoadFrom(cfgFile)
		if err != nil {
			return err
		}

		// Override vault path from flag
		if vaultPath != "" {
			cfg.VaultPath = vaultPath
		}

		// For config command without args, don't require valid vault
		if cmd.Name() == "config" {
			return nil
		}

		// Validate config for other commands
		if err := cfg.Validate(); err != nil {
			return err
		}

		// Create vault instance
		expandedPath, err := cfg.ExpandedVaultPath()
		if err != nil {
			return err
		}
		vlt = vault.New(expandedPath)

		// Set up versioning if enabled and vault is a git repo
		if cfg.VersioningEnabled() && versioning.IsAvailable() {
			g := versioning.New(expandedPath)
			if g.IsRepo() {
				vlt.SetVersioning(g)
			}
		}

		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func init() {
	// Check NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		noColor = true
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.config/ruin/config.yml)")
	rootCmd.PersistentFlags().StringVar(&vaultPath, "vault", "", "vault path (overrides config)")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")

	// Add commands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(commands.NewLogCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewSearchCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewUpdateCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewInitCmd(&jsonOut))
	rootCmd.AddCommand(commands.NewConfigCmd(&jsonOut))
	rootCmd.AddCommand(commands.NewDoctorCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewQueryCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewTodayCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewYesterdayCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewTagsCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewGetCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewParentCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewSuggestCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewComposeCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewPickCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewNoteCmd(getVault, &jsonOut))
	rootCmd.AddCommand(commands.NewDevCmd(&jsonOut))
}

// getVault returns the current vault instance.
func getVault() *vault.Vault {
	return vlt
}

// exitCode constants for consistent exit behavior
const (
	exitSuccess     = 0
	exitError       = 1
	exitUsageError  = 2
	exitUserAborted = 3
)

// printError writes an error message to stderr
func printError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
}

// sanitizeLeadingDashArgs fixes a cobra bug where positional arguments starting
// with "- " (dash-space, e.g. markdown list items) are treated as flags.
// See https://github.com/spf13/cobra/issues/2295
//
// Only applies to "log" since its content arg commonly starts with "- ".
// The fix removes the "- " arg from its position and appends it after a "--"
// separator at the end, so flags in any position still get parsed correctly.
func sanitizeLeadingDashArgs() {
	// Find "log" subcommand
	logIdx := -1
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--" {
			return
		}
		if os.Args[i] == "log" && !strings.HasPrefix(os.Args[i], "-") {
			logIdx = i
			break
		}
	}
	if logIdx < 0 {
		return
	}

	// After "log", find the first arg starting with "- " (dash-space)
	dashArgIdx := -1
	for i := logIdx + 1; i < len(os.Args); i++ {
		if os.Args[i] == "--" {
			return // explicit separator already present
		}
		if strings.HasPrefix(os.Args[i], "- ") {
			dashArgIdx = i
			break
		}
	}
	if dashArgIdx < 0 {
		return
	}

	// Remove the "- " arg and re-append it after "--" at the end.
	// This keeps all flags (before or after) parseable.
	dashArg := os.Args[dashArgIdx]
	newArgs := make([]string, 0, len(os.Args)+1)
	newArgs = append(newArgs, os.Args[:dashArgIdx]...)
	newArgs = append(newArgs, os.Args[dashArgIdx+1:]...)
	newArgs = append(newArgs, "--", dashArg)
	os.Args = newArgs
}
