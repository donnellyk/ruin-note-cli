package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/donnellyk/ruin-note-cli/pkg/notetext"
	"github.com/spf13/cobra"
)

type extractOutput struct {
	Global []string `json:"global"`
	Inline []string `json:"inline"`
}

func newLogExtractCmd(jsonOutput *bool) *cobra.Command {
	var title string

	cmd := &cobra.Command{
		Use:   "extract [content]",
		Short: "Extract and classify tags from content",
		Long: `Extract tags from the provided content and classify them as global or inline.

Global tags appear on tag-only lines (lines containing only tags and separators).
Inline tags appear on lines that also contain non-tag content.

Content can be provided as a positional argument or via stdin.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := getContent(args, false)
			if err != nil {
				return err
			}

			global, inline := notetext.ClassifyTags(content, title)

			if *jsonOutput {
				out := extractOutput{
					Global: nonNil(global),
					Inline: nonNil(inline),
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Println("global:")
			for _, tag := range global {
				fmt.Printf("  %s\n", tag)
			}
			fmt.Println("inline:")
			for _, tag := range inline {
				fmt.Printf("  %s\n", tag)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "title for tag classification context")
	return cmd
}

// nonNil returns the slice or an empty non-nil slice (for consistent JSON output).
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
