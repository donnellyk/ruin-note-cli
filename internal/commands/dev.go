package commands

import (
	"github.com/spf13/cobra"
)

// NewDevCmd creates the hidden dev command group for developer utilities.
func NewDevCmd(jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "dev",
		Short:  "Developer utilities",
		Hidden: true,
	}

	cmd.AddCommand(newSeedCmd(jsonOutput))

	return cmd
}
