package commands

import (
	"fmt"
	"os"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
)

// RefreshLinkedCards extracts wiki links from the note's content,
// resolves titles to UUIDs via the titles index, and updates LinkedCards.
// Unresolvable links produce a stderr warning and are omitted.
func RefreshLinkedCards(n *note.Note, index *vault.TitlesIndex) {
	titles := note.ExtractWikiLinkTitles(n.Content)
	if len(titles) == 0 {
		n.LinkedCards = nil
		return
	}

	seen := make(map[string]bool)
	var resolved []string

	for _, title := range titles {
		uuid, found := index.FindByTitle(title)
		if !found {
			fmt.Fprintf(os.Stderr, "warning: wiki link [[%s]] could not be resolved\n", title)
			continue
		}
		if !seen[uuid] {
			seen[uuid] = true
			resolved = append(resolved, uuid)
		}
	}

	n.LinkedCards = resolved
}
