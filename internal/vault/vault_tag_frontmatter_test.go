package vault

import "testing"

func TestVault_TagFrontmatterDefault(t *testing.T) {
	v := New(t.TempDir())
	if !v.TagFrontmatterEnabled() {
		t.Errorf("TagFrontmatterEnabled() default = false, want true")
	}
}

func TestVault_SetTagFrontmatter(t *testing.T) {
	v := New(t.TempDir())
	v.SetTagFrontmatter(false)
	if v.TagFrontmatterEnabled() {
		t.Errorf("after SetTagFrontmatter(false), TagFrontmatterEnabled() = true")
	}
	v.SetTagFrontmatter(true)
	if !v.TagFrontmatterEnabled() {
		t.Errorf("after SetTagFrontmatter(true), TagFrontmatterEnabled() = false")
	}
}
