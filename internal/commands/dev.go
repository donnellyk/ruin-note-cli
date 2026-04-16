package commands

import (
	"github.com/spf13/cobra"
)

func NewDevCmd(jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "dev",
		Short:  "Developer utilities",
		Hidden: true,
	}

	cmd.AddCommand(newSeedCmd(jsonOutput))

	return cmd
}
