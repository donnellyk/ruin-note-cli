package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// findAliasWithPrefix searches aliases for one with a case-insensitive prefix match.
// Returns the matched alias name or empty string if no match found.
func findAliasWithPrefix(aliases []string, prefix string) string {
	for _, alias := range aliases {
		if strings.HasPrefix(strings.ToLower(alias), prefix) {
			return alias
		}
	}
	return ""
}

type suggestion struct {
	UUID    string `json:"uuid"`
	Title   string `json:"title"`
	Path    string `json:"path"`
	Parent  string `json:"parent,omitempty"`
	Alias   string `json:"alias,omitempty"`
	Display string `json:"display"`
}

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
				return suggestByVaultScan(vlt, prefix, limit, *jsonOutput)
			}

			var results []suggestion
			seen := make(map[string]bool)

			for uuid, entry := range index.Titles {
				if seen[uuid] {
					continue
				}
				titleLower := strings.ToLower(entry.Title)

				if strings.HasPrefix(titleLower, prefix) {
					seen[uuid] = true
					results = append(results, suggestion{
						UUID:    uuid,
						Title:   entry.Title,
						Path:    entry.Path,
						Parent:  entry.Parent,
						Display: entry.Title,
					})
				} else if matchedAlias := findAliasWithPrefix(entry.Aliases, prefix); matchedAlias != "" {
					seen[uuid] = true
					results = append(results, suggestion{
						UUID:    uuid,
						Title:   entry.Title,
						Path:    entry.Path,
						Parent:  entry.Parent,
						Alias:   matchedAlias,
						Display: entry.Title + " (alias: " + matchedAlias + ")",
					})
				}
			}

			return outputSuggestions(results, limit, *jsonOutput)
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

	var results []suggestion
	for _, path := range paths {
		n, err := note.Load(path)
		if err != nil {
			continue
		}
		titleLower := strings.ToLower(n.Title)

		if strings.HasPrefix(titleLower, prefix) {
			results = append(results, suggestion{
				UUID:    n.UUID,
				Title:   n.Title,
				Path:    n.FilePath,
				Display: n.Title,
			})
		} else if matchedAlias := findAliasWithPrefix(n.Aliases, prefix); matchedAlias != "" {
			results = append(results, suggestion{
				UUID:    n.UUID,
				Title:   n.Title,
				Path:    n.FilePath,
				Alias:   matchedAlias,
				Display: n.Title + " (alias: " + matchedAlias + ")",
			})
		}
	}

	return outputSuggestions(results, limit, jsonOutput)
}

func outputSuggestions(results []suggestion, limit int, jsonOutput bool) error {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Display < results[j].Display
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
		fmt.Printf("%s\t%s\n", r.UUID, r.Display)
	}
	return nil
}
