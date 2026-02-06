package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// NewSuggestCmd creates the suggest command.
func NewSuggestCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "suggest <prefix>",
		Short: "Suggest notes by title prefix",
		Long: `Find notes whose titles match the given prefix (case-insensitive).

Uses the titles index for fast lookup. Falls back to vault scan if the
index is missing.`,
		Example: `  ruin suggest "Sprint"
  ruin suggest "meet" --limit 5
  ruin suggest "proj" --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			prefix := strings.ToLower(args[0])

			index, err := vlt.LoadTitles()
			if err != nil || len(index.Titles) == 0 {
				// Fallback to vault scan
				return suggestByVaultScan(vlt, prefix, limit, *jsonOutput)
			}

			type suggestion struct {
				UUID   string `json:"uuid"`
				Title  string `json:"title"`
				Path   string `json:"path"`
				Parent string `json:"parent,omitempty"`
			}

			var results []suggestion
			for uuid, entry := range index.Titles {
				if strings.HasPrefix(strings.ToLower(entry.Title), prefix) {
					results = append(results, suggestion{
						UUID:   uuid,
						Title:  entry.Title,
						Path:   entry.Path,
						Parent: entry.Parent,
					})
				}
			}

			// Sort by title
			sort.Slice(results, func(i, j int) bool {
				return results[i].Title < results[j].Title
			})

			// Apply limit
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if *jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			for _, r := range results {
				fmt.Printf("%s\t%s\n", r.UUID, r.Title)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "max results")
	return cmd
}

func suggestByVaultScan(vlt *vault.Vault, prefix string, limit int, jsonOutput bool) error {
	paths, err := vlt.ListNotes()
	if err != nil {
		return err
	}

	type suggestion struct {
		UUID  string `json:"uuid"`
		Title string `json:"title"`
		Path  string `json:"path"`
	}

	var results []suggestion
	for _, path := range paths {
		n, err := note.Load(path)
		if err != nil {
			continue
		}
		if strings.HasPrefix(strings.ToLower(n.Title), prefix) {
			results = append(results, suggestion{
				UUID:  n.UUID,
				Title: n.Title,
				Path:  n.FilePath,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Title < results[j].Title
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	for _, r := range results {
		fmt.Printf("%s\t%s\n", r.UUID, r.Title)
	}
	return nil
}
