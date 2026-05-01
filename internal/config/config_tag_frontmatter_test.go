package config

import "testing"

func TestConfig_TagFrontmatterEnabled(t *testing.T) {
	t.Run("default is true", func(t *testing.T) {
		cfg := &Config{}
		if !cfg.TagFrontmatterEnabled() {
			t.Errorf("default TagFrontmatterEnabled() = false, want true")
		}
	})

	t.Run("false in config wins when no env", func(t *testing.T) {
		f := false
		cfg := &Config{TagFrontmatter: &f}
		if cfg.TagFrontmatterEnabled() {
			t.Errorf("TagFrontmatter=false config still reports enabled")
		}
	})

	t.Run("true in config wins when no env", func(t *testing.T) {
		tr := true
		cfg := &Config{TagFrontmatter: &tr}
		if !cfg.TagFrontmatterEnabled() {
			t.Errorf("TagFrontmatter=true config reports disabled")
		}
	})

	t.Run("env var false overrides config true", func(t *testing.T) {
		tr := true
		cfg := &Config{TagFrontmatter: &tr}
		t.Setenv("RUIN_TAG_FRONTMATTER", "false")
		if cfg.TagFrontmatterEnabled() {
			t.Errorf("RUIN_TAG_FRONTMATTER=false did not override")
		}
	})

	t.Run("env var 0 overrides config true", func(t *testing.T) {
		tr := true
		cfg := &Config{TagFrontmatter: &tr}
		t.Setenv("RUIN_TAG_FRONTMATTER", "0")
		if cfg.TagFrontmatterEnabled() {
			t.Errorf("RUIN_TAG_FRONTMATTER=0 did not override")
		}
	})
}
