package commands

import (
	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

// serializeOptionsForVault derives Frontmatter SerializeOptions from the
// vault's tag_frontmatter setting. When the flag is on (default), the vault
// save path emits both tags: and inline-tags:. When off, both fields are
// stripped from frontmatter on save.
func serializeOptionsForVault(vlt *vault.Vault) note.SerializeOptions {
	if vlt == nil || vlt.TagFrontmatterEnabled() {
		return note.SerializeOptions{EmitInlineTags: true}
	}
	return note.SerializeOptions{SkipOwnTagMirror: true}
}

// saveNoteForVault writes a note honoring the vault's tag_frontmatter
// setting. All vault-aware production save sites go through this helper so
// the flag's contract is enforced in one place.
func saveNoteForVault(n *note.Note, vlt *vault.Vault) error {
	return n.SaveWithOptions(serializeOptionsForVault(vlt))
}

// ownTagMirrorWillChange reports whether saving n with the vault's current
// tag_frontmatter setting would add or remove tags:/inline-tags: keys
// relative to what's on disk (rawFM). Used by doctor so users flipping the
// flag see the per-note frontmatter delta in --dry-run output.
func ownTagMirrorWillChange(rawFM *note.Frontmatter, n *note.Note, vlt *vault.Vault) bool {
	if rawFM == nil || n == nil {
		return false
	}
	flagOn := vlt == nil || vlt.TagFrontmatterEnabled()
	hadTags := len(rawFM.Tags) > 0
	hadInline := len(rawFM.InlineTags) > 0
	willWriteTags := flagOn && len(n.Tags) > 0
	willWriteInline := flagOn && len(n.InlineTags) > 0
	return hadTags != willWriteTags || hadInline != willWriteInline
}
