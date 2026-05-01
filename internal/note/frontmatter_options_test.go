package note

import (
	"strings"
	"testing"
)

func TestSerializeWithOptions_DefaultMatchesNoArgs(t *testing.T) {
	fm := &Frontmatter{
		UUID:          "abc",
		Tags:          []string{"daily"},
		InlineTags:    []string{"followup"},
		InheritedTags: []string{"project"},
	}

	defaultOut, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	optsOut, err := fm.SerializeWithOptions(SerializeOptions{})
	if err != nil {
		t.Fatalf("SerializeWithOptions: %v", err)
	}
	if defaultOut != optsOut {
		t.Errorf("Serialize() and SerializeWithOptions(zero) diverged:\n--default--\n%s\n--opts--\n%s", defaultOut, optsOut)
	}
	if strings.Contains(defaultOut, "inline-tags:") {
		t.Errorf("default Serialize emitted inline-tags: (should preserve v0.4.0 no-args contract)\n%s", defaultOut)
	}
	if !strings.Contains(defaultOut, "tags:") {
		t.Errorf("default Serialize missing tags:\n%s", defaultOut)
	}
	if !strings.Contains(defaultOut, "inherited-tags:") {
		t.Errorf("default Serialize missing inherited-tags:\n%s", defaultOut)
	}
}

func TestSerializeWithOptions_EmitInlineTags(t *testing.T) {
	fm := &Frontmatter{
		UUID:          "abc",
		Tags:          []string{"daily"},
		InlineTags:    []string{"followup"},
		InheritedTags: []string{"project"},
	}

	out, err := fm.SerializeWithOptions(SerializeOptions{EmitInlineTags: true})
	if err != nil {
		t.Fatalf("SerializeWithOptions: %v", err)
	}
	for _, want := range []string{"tags:", "inline-tags:", "inherited-tags:"} {
		if !strings.Contains(out, want) {
			t.Errorf("EmitInlineTags=true output missing %q:\n%s", want, out)
		}
	}
}

func TestSerializeWithOptions_SkipOwnTagMirror(t *testing.T) {
	// SkipOwnTagMirror at the Frontmatter layer suppresses inline-tags: but
	// writes whatever Tags the caller put on fm — Note.SerializeWithOptions
	// is responsible for substituting FrontmatterTags so Obsidian-set
	// `tags:` is preserved.
	fm := &Frontmatter{
		UUID:          "abc",
		Tags:          []string{"daily"},
		InlineTags:    []string{"followup"},
		InheritedTags: []string{"project"},
	}

	out, err := fm.SerializeWithOptions(SerializeOptions{
		EmitInlineTags:   true,
		SkipOwnTagMirror: true,
	})
	if err != nil {
		t.Fatalf("SerializeWithOptions: %v", err)
	}
	if !HasFrontmatterKey(out, "tags") {
		t.Errorf("SkipOwnTagMirror=true should write fm.Tags as-is:\n%s", out)
	}
	if HasFrontmatterKey(out, "inline-tags") {
		t.Errorf("SkipOwnTagMirror=true emitted inline-tags:\n%s", out)
	}
	if !HasFrontmatterKey(out, "inherited-tags") {
		t.Errorf("SkipOwnTagMirror=true should still emit inherited-tags:\n%s", out)
	}
}

// TestSerializeWithOptions_SkipOwnTagMirror_NilTagsDropsKey — when fm.Tags is
// empty (caller's FrontmatterTags was nil because the file had no `tags:`),
// SkipOwnTagMirror must not introduce a `tags:` key.
func TestSerializeWithOptions_SkipOwnTagMirror_NilTagsDropsKey(t *testing.T) {
	fm := &Frontmatter{
		UUID:          "abc",
		InlineTags:    []string{"followup"},
		InheritedTags: []string{"project"},
	}

	out, err := fm.SerializeWithOptions(SerializeOptions{
		EmitInlineTags:   true,
		SkipOwnTagMirror: true,
	})
	if err != nil {
		t.Fatalf("SerializeWithOptions: %v", err)
	}
	if HasFrontmatterKey(out, "tags") {
		t.Errorf("nil Tags + SkipOwnTagMirror should not introduce tags:\n%s", out)
	}
}

func TestSerializeWithOptions_PreservesNodeForm(t *testing.T) {
	// When a Frontmatter was parsed from disk, serializeFromNode is the path
	// used. Verify SkipOwnTagMirror strips inline-tags: but leaves tags: as-is
	// (Note.SerializeWithOptions handles preserving the on-disk value).
	src := `---
uuid: abc
tags:
  - daily
inline-tags:
  - followup
inherited-tags:
  - project
custom: keep
---
body
`
	fm, _, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}

	out, err := fm.SerializeWithOptions(SerializeOptions{SkipOwnTagMirror: true})
	if err != nil {
		t.Fatalf("SerializeWithOptions: %v", err)
	}
	if !HasFrontmatterKey(out, "tags") {
		t.Errorf("SkipOwnTagMirror=true should preserve tags: in node form:\n%s", out)
	}
	if HasFrontmatterKey(out, "inline-tags") {
		t.Errorf("SkipOwnTagMirror=true did not strip inline-tags: from node form:\n%s", out)
	}
	if !HasFrontmatterKey(out, "inherited-tags") {
		t.Errorf("inherited-tags: dropped from node form:\n%s", out)
	}
	if !strings.Contains(out, "custom:") {
		t.Errorf("custom user field dropped:\n%s", out)
	}
}

func TestHasLegacyTagFrontmatter_StrippedInlineTagsNotFlagged(t *testing.T) {
	// A v0.4.0+ note with `inline-tags:` in stripped form is current format;
	// it must not be flagged as legacy by the migration detector.
	current := `---
uuid: abc
tags:
  - daily
inline-tags:
  - followup
---
body
`
	if HasLegacyTagFrontmatter(current) {
		t.Errorf("stripped inline-tags should not be flagged as legacy:\n%s", current)
	}
}

func TestHasLegacyTagFrontmatter_HashPrefixedInlineTagsFlagged(t *testing.T) {
	legacy := `---
uuid: abc
tags:
  - "#daily"
inline-tags:
  - "#followup"
---
body
`
	if !HasLegacyTagFrontmatter(legacy) {
		t.Errorf("`#`-prefixed tags should be flagged as legacy:\n%s", legacy)
	}
}

func TestHasLegacyTagFrontmatter_InheritedTagsHashFlagged(t *testing.T) {
	legacy := `---
uuid: abc
tags:
  - daily
inherited-tags:
  - "#project"
---
body
`
	if !HasLegacyTagFrontmatter(legacy) {
		t.Errorf("`#`-prefixed inherited-tags should be flagged as legacy:\n%s", legacy)
	}
}
