package note

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const TimeFormat = "2006-01-02T15:04:05-07:00"

type Note struct {
	UUID          string
	Created       time.Time
	Updated       time.Time
	Tags          []string
	InlineTags    []string
	InheritedTags []string
	Dates         []string
	Parent        string
	Order         *int
	LinkedCards   []string
	URL           string
	Title         string
	Content       string
	FilePath      string

	// Extra preserves user-added frontmatter fields round-trip; unknown keys
	// here are re-emitted verbatim on save.
	Extra map[string]any
}

var headerPattern = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)

func Parse(content string) (*Note, error) {
	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	note := &Note{
		UUID:          fm.UUID,
		Parent:        fm.Parent,
		Order:         fm.Order,
		LinkedCards:   fm.LinkedCards,
		URL:           fm.URL,
		InheritedTags: fm.InheritedTags,
		Content:       body,
		Extra:         fm.Extra,
	}

	if fm.Created != "" {
		if t, err := time.Parse(TimeFormat, fm.Created); err == nil {
			note.Created = t
		}
	}
	if fm.Updated != "" {
		if t, err := time.Parse(TimeFormat, fm.Updated); err == nil {
			note.Updated = t
		}
	}

	note.Title = extractTitle(body)

	globalTags, inlineTags := ClassifyTags(body, note.Title)
	note.Tags = globalTags
	note.InlineTags = inlineTags

	note.Dates = ExtractDates(body)

	return note, nil
}

func Load(path string) (*Note, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	note, err := Parse(string(data))
	if err != nil {
		return nil, err
	}

	note.FilePath = path
	return note, nil
}

func extractTitle(content string) string {
	match := headerPattern.FindStringSubmatch(content)
	if len(match) >= 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func (n *Note) Serialize() (string, error) {
	return n.SerializeWithOptions(SerializeOptions{})
}

// SerializeWithOptions threads SerializeOptions through to the underlying
// Frontmatter serializer. Vault-aware callers use this via
// commands.saveNoteForVault to honor the tag_frontmatter setting; direct
// callers (tests, downstream Go consumers) get the v0.4.0 default by passing
// the zero value.
func (n *Note) SerializeWithOptions(opts SerializeOptions) (string, error) {
	fm := &Frontmatter{
		UUID:          n.UUID,
		Tags:          n.Tags,
		InlineTags:    n.InlineTags,
		InheritedTags: n.InheritedTags,
		Dates:         n.Dates,
		Parent:        n.Parent,
		Order:         n.Order,
		LinkedCards:   n.LinkedCards,
		URL:           n.URL,
		Extra:         n.Extra,
	}

	if !n.Created.IsZero() {
		fm.Created = n.Created.Format(TimeFormat)
	}
	if !n.Updated.IsZero() {
		fm.Updated = n.Updated.Format(TimeFormat)
	}

	fmStr, err := fm.SerializeWithOptions(opts)
	if err != nil {
		return "", fmt.Errorf("failed to serialize frontmatter: %w", err)
	}

	return fmStr + n.Content, nil
}

func (n *Note) Save() error {
	return n.SaveWithOptions(SerializeOptions{})
}

// SaveWithOptions writes the note to disk with the supplied serialize options.
// Vault save callers use commands.saveNoteForVault, which derives opts from
// the vault's tag_frontmatter setting.
func (n *Note) SaveWithOptions(opts SerializeOptions) error {
	if n.FilePath == "" {
		return fmt.Errorf("note has no file path")
	}

	content, err := n.SerializeWithOptions(opts)
	if err != nil {
		return err
	}

	dir := filepath.Dir(n.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(n.FilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (n *Note) EnsureUUID() {
	if n.UUID == "" {
		n.UUID = uuid.New().String()
	}
}

func (n *Note) SetTimestamps() {
	now := time.Now()
	if n.Created.IsZero() {
		n.Created = now
	}
	n.Updated = now
}

func (n *Note) RefreshTags() {
	globalTags, inlineTags := ClassifyTags(n.Content, n.Title)
	n.Tags = globalTags
	n.InlineTags = inlineTags
}

func (n *Note) RefreshDates() {
	n.Dates = ExtractDates(n.Content)
}

// EffectiveGlobalTags returns own global tags merged with inherited tags (deduplicated).
func (n *Note) EffectiveGlobalTags() []string {
	return MergeTags(n.Tags, n.InheritedTags)
}

// AllTags returns global + inline merged, deduplicated. Does NOT include
// inherited tags (parent already counts them).
func (n *Note) AllTags() []string {
	return MergeTags(n.Tags, n.InlineTags)
}

func (n *Note) GenerateFilename() string {
	if n.Title != "" {
		return SanitizeFilename(n.Title)
	}

	t := n.Created
	if t.IsZero() {
		t = time.Now()
	}
	return t.Format("2006-01-02T15-04-05")
}

func SanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
	)
	name = replacer.Replace(name)

	name = strings.TrimSpace(name)
	name = strings.Trim(name, ".")

	if len(name) > 200 {
		name = name[:200]
	}

	return name
}

func (n *Note) ContentWithoutTitle() string {
	if n.Title == "" {
		return n.Content
	}

	lines := strings.Split(n.Content, "\n")
	var result []string
	foundTitle := false

	for _, line := range lines {
		if !foundTitle && headerPattern.MatchString(line) {
			foundTitle = true
			continue
		}
		result = append(result, line)
	}

	for len(result) > 0 && strings.TrimSpace(result[0]) == "" {
		result = result[1:]
	}

	return strings.Join(result, "\n")
}
