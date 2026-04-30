package note

import (
	"strings"
	"testing"
)

// TestRoundTrip_PreservesKeyOrder verifies that re-saving foreign frontmatter
// (typed fields interleaved with custom Obsidian-style keys) keeps the original
// key order. Today's map-based encoder sorts alphabetically; Node-based encoder
// must keep the source order.
func TestRoundTrip_PreservesKeyOrder(t *testing.T) {
	src := `aliases:
  - "Old Name"
uuid: abc-123
custom_field: hello
tags:
  - "#work"
publish: true
`

	fm, err := parseFrontmatterYAML(src)
	if err != nil {
		t.Fatalf("parseFrontmatterYAML err = %v", err)
	}

	got, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize err = %v", err)
	}

	keys := []string{"aliases", "uuid", "custom_field", "tags", "publish"}
	lastIdx := -1
	for _, k := range keys {
		idx := strings.Index(got, k+":")
		if idx == -1 {
			t.Errorf("serialized output missing key %q\n%s", k, got)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("key %q appeared out of source order (idx %d, prev %d)\n%s", k, idx, lastIdx, got)
		}
		lastIdx = idx
	}
}

// TestRoundTrip_PreservesComments verifies head/foot/inline comments survive a
// no-mutation round trip. yaml.v3 stores them on Node; the Node-based serializer
// emits them; the map-based serializer would drop them.
func TestRoundTrip_PreservesComments(t *testing.T) {
	src := `# top of file
uuid: abc-123  # inline
# describes tags below
tags:
  - "#work"
`

	fm, err := parseFrontmatterYAML(src)
	if err != nil {
		t.Fatalf("parseFrontmatterYAML err = %v", err)
	}

	got, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize err = %v", err)
	}

	want := []string{"# top of file", "# inline", "# describes tags below"}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("serialized output missing comment %q\n%s", w, got)
		}
	}
}

// TestRoundTrip_PreservesScalarStyleOnUnchangedExtra verifies that a custom
// Extra field with a specific quote style keeps that style when other fields
// change but the Extra value does not.
func TestRoundTrip_PreservesScalarStyleOnUnchangedExtra(t *testing.T) {
	src := `uuid: abc-123
description: "Quoted description"
`

	fm, err := parseFrontmatterYAML(src)
	if err != nil {
		t.Fatalf("parseFrontmatterYAML err = %v", err)
	}

	// Mutate a typed field; Extra stays untouched.
	fm.UUID = "new-uuid"

	got, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize err = %v", err)
	}

	if !strings.Contains(got, `"Quoted description"`) {
		t.Errorf("expected double-quoted style on unchanged Extra value, got:\n%s", got)
	}
	if !strings.Contains(got, "uuid: new-uuid") {
		t.Errorf("expected updated uuid, got:\n%s", got)
	}
}

// TestRoundTrip_AppendsNewTypedFieldAtEnd verifies that adding a typed field
// not present in the source mapping appends it at the end (rather than guessing
// at a canonical position) — preserves the user's original layout for keys
// they wrote.
func TestRoundTrip_AppendsNewTypedFieldAtEnd(t *testing.T) {
	src := `uuid: abc-123
custom: x
`

	fm, err := parseFrontmatterYAML(src)
	if err != nil {
		t.Fatalf("parseFrontmatterYAML err = %v", err)
	}

	fm.Tags = []string{"#new"}

	got, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize err = %v", err)
	}

	uuidIdx := strings.Index(got, "uuid:")
	customIdx := strings.Index(got, "custom:")
	tagsIdx := strings.Index(got, "tags:")
	if !(uuidIdx < customIdx && customIdx < tagsIdx) {
		t.Errorf("expected order uuid -> custom -> tags, got:\n%s", got)
	}
}

// TestRoundTrip_RemovesClearedTypedField verifies that clearing a typed field
// removes the corresponding key from the Node on Serialize.
func TestRoundTrip_RemovesClearedTypedField(t *testing.T) {
	src := `uuid: abc-123
parent: parent-uuid
custom: x
`

	fm, err := parseFrontmatterYAML(src)
	if err != nil {
		t.Fatalf("parseFrontmatterYAML err = %v", err)
	}

	fm.Parent = ""

	got, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize err = %v", err)
	}

	if strings.Contains(got, "parent:") {
		t.Errorf("expected parent: to be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "uuid: abc-123") || !strings.Contains(got, "custom: x") {
		t.Errorf("expected uuid and custom to remain, got:\n%s", got)
	}
}

// TestRoundTrip_RemovesDeletedExtraKey verifies that deleting an Extra key
// removes the corresponding mapping entry on Serialize.
func TestRoundTrip_RemovesDeletedExtraKey(t *testing.T) {
	src := `uuid: abc-123
custom_a: keep
custom_b: drop
`

	fm, err := parseFrontmatterYAML(src)
	if err != nil {
		t.Fatalf("parseFrontmatterYAML err = %v", err)
	}

	delete(fm.Extra, "custom_b")

	got, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize err = %v", err)
	}

	if strings.Contains(got, "custom_b") {
		t.Errorf("expected custom_b to be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "custom_a") {
		t.Errorf("expected custom_a to remain, got:\n%s", got)
	}
}

// TestRoundTrip_AddsNewExtraKey verifies that adding a key to Extra appends a
// new entry to the Node.
func TestRoundTrip_AddsNewExtraKey(t *testing.T) {
	src := `uuid: abc-123
custom_a: keep
`

	fm, err := parseFrontmatterYAML(src)
	if err != nil {
		t.Fatalf("parseFrontmatterYAML err = %v", err)
	}

	fm.Extra["custom_b"] = "added"

	got, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize err = %v", err)
	}

	if !strings.Contains(got, "custom_b:") {
		t.Errorf("expected custom_b: to be added, got:\n%s", got)
	}
	// Should be appended after existing keys.
	customAIdx := strings.Index(got, "custom_a")
	customBIdx := strings.Index(got, "custom_b")
	if customAIdx > customBIdx {
		t.Errorf("expected custom_b to be appended after custom_a, got:\n%s", got)
	}
}

// TestObsidianFixture_RoundTripPreservesShape exercises the realistic migration
// case: an Obsidian note with aliases, cssclass, publish, tags. Values must
// round-trip; key order must be preserved.
func TestObsidianFixture_RoundTripPreservesShape(t *testing.T) {
	src := `aliases:
  - Daily 2024-01-15
cssclass: daily
publish: false
tags:
  - daily
  - work
`

	fm, err := parseFrontmatterYAML(src)
	if err != nil {
		t.Fatalf("parseFrontmatterYAML err = %v", err)
	}

	got, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize err = %v", err)
	}

	// Re-parse to verify values round-tripped.
	fm2, err := parseFrontmatterYAML(strings.TrimPrefix(strings.TrimSuffix(got, "---\n"), "---\n"))
	if err != nil {
		t.Fatalf("re-parse err = %v\noutput:\n%s", err, got)
	}

	if len(fm2.Tags) != 2 || fm2.Tags[0] != "daily" || fm2.Tags[1] != "work" {
		t.Errorf("Tags = %v, want [daily, work]", fm2.Tags)
	}
	if v, _ := fm2.Extra["aliases"]; v == nil {
		t.Errorf("aliases lost from Extra after round-trip; got %v", fm2.Extra)
	}
	if v, ok := fm2.Extra["cssclass"]; !ok || v != "daily" {
		t.Errorf("cssclass = %v, want daily", v)
	}
	if v, ok := fm2.Extra["publish"]; !ok || v != false {
		t.Errorf("publish = %v (%T), want false", v, v)
	}

	// Order check: aliases -> cssclass -> publish -> tags.
	keys := []string{"aliases:", "cssclass:", "publish:", "tags:"}
	lastIdx := -1
	for _, k := range keys {
		idx := strings.Index(got, k)
		if idx == -1 {
			t.Errorf("serialized output missing %q\n%s", k, got)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("key %q out of order (idx %d, prev %d)\n%s", k, idx, lastIdx, got)
		}
		lastIdx = idx
	}
}

// TestSerializeWithoutNode_LegacyPathStillWorks verifies that a Frontmatter
// constructed in code (no source YAML) still serializes via the map-based path.
func TestSerializeWithoutNode_LegacyPathStillWorks(t *testing.T) {
	fm := &Frontmatter{
		UUID: "abc",
		Tags: []string{"#x"},
	}
	got, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize err = %v", err)
	}
	if !strings.Contains(got, "uuid: abc") {
		t.Errorf("missing uuid, got:\n%s", got)
	}
	if !strings.Contains(got, "#x") {
		t.Errorf("missing tag, got:\n%s", got)
	}
}
