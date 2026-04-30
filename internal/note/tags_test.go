package note

import (
	"reflect"
	"testing"
)

// Smoke tests to verify wrappers delegate correctly to pkg/notetext.
// Comprehensive tests live in pkg/notetext/tags_test.go.

func TestExtractTags_Smoke(t *testing.T) {
	tags := ExtractTags("Has #foo and #bar tags.")
	expected := []string{"#foo", "#bar"}
	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("ExtractTags() = %v, want %v", tags, expected)
	}
}

func TestClassifyTags_Smoke(t *testing.T) {
	content := "#global\n\nText with #inline here."
	global, inline := ClassifyTags(content, "")

	if !reflect.DeepEqual(global, []string{"global"}) {
		t.Errorf("global = %v, want [global]", global)
	}
	if !reflect.DeepEqual(inline, []string{"inline"}) {
		t.Errorf("inline = %v, want [inline]", inline)
	}
}

func TestNormalizeStored_Smoke(t *testing.T) {
	if got := NormalizeStored("#Foo"); got != "foo" {
		t.Errorf("NormalizeStored(#Foo) = %q, want foo", got)
	}
}
