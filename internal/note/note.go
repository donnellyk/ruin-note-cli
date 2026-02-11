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

// TimeFormat is the format used for created/updated timestamps.
const TimeFormat = "2006-01-02T15:04:05-07:00"

// Note represents a markdown note with frontmatter.
type Note struct {
	UUID       string
	Created    time.Time
	Updated    time.Time
	Tags       []string // Global tags (categorization)
	InlineTags []string // Inline tags (contextual annotations within content)
	Parent     string   // UUID of parent note
	Order       *int     // Manual sort order (nil = unset)
	LinkedCards []string // Resolved UUIDs from [[wiki links]]
	Title       string   // H1 header text (without #)
	Content    string   // Full markdown content (without frontmatter)
	FilePath   string   // Path to the file on disk

	// Extra preserves additional frontmatter fields added by the user.
	Extra map[string]interface{}
}

// headerPattern matches any markdown header (H1 through H6).
var headerPattern = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)

// Parse reads a note from markdown content.
// It extracts frontmatter, title, and tags.
func Parse(content string) (*Note, error) {
	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	note := &Note{
		UUID:        fm.UUID,
		Parent:      fm.Parent,
		Order:       fm.Order,
		LinkedCards: fm.LinkedCards,
		Content:     body,
		Extra:       fm.Extra,
	}

	// Parse timestamps
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

	// Extract title from first header
	note.Title = extractTitle(body)

	// Extract and classify tags
	globalTags, inlineTags := ClassifyTags(body, note.Title)
	note.Tags = globalTags
	note.InlineTags = inlineTags

	return note, nil
}

// Load reads a note from a file.
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

// extractTitle finds the first markdown header (any level) in the content.
func extractTitle(content string) string {
	match := headerPattern.FindStringSubmatch(content)
	if len(match) >= 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// Serialize converts the note back to markdown with frontmatter.
func (n *Note) Serialize() (string, error) {
	fm := &Frontmatter{
		UUID:        n.UUID,
		Tags:        n.Tags,
		InlineTags:  n.InlineTags,
		Parent:      n.Parent,
		Order:       n.Order,
		LinkedCards: n.LinkedCards,
		Extra:       n.Extra,
	}

	if !n.Created.IsZero() {
		fm.Created = n.Created.Format(TimeFormat)
	}
	if !n.Updated.IsZero() {
		fm.Updated = n.Updated.Format(TimeFormat)
	}

	fmStr, err := fm.Serialize()
	if err != nil {
		return "", fmt.Errorf("failed to serialize frontmatter: %w", err)
	}

	return fmStr + n.Content, nil
}

// Save writes the note to a file.
// If the note has no FilePath set, an error is returned.
func (n *Note) Save() error {
	if n.FilePath == "" {
		return fmt.Errorf("note has no file path")
	}

	content, err := n.Serialize()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(n.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(n.FilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// EnsureUUID generates a UUID if one is not already set.
func (n *Note) EnsureUUID() {
	if n.UUID == "" {
		n.UUID = uuid.New().String()
	}
}

// SetTimestamps sets created (if not set) and updated timestamps.
func (n *Note) SetTimestamps() {
	now := time.Now()
	if n.Created.IsZero() {
		n.Created = now
	}
	n.Updated = now
}

// RefreshTags re-extracts tags from the content.
func (n *Note) RefreshTags() {
	globalTags, inlineTags := ClassifyTags(n.Content, n.Title)
	n.Tags = globalTags
	n.InlineTags = inlineTags
}

// AllTags returns all tags (global + inline merged, deduplicated).
// Use this for tag index operations where both types should be counted.
func (n *Note) AllTags() []string {
	return MergeTags(n.Tags, n.InlineTags)
}

// GenerateFilename creates a filename for the note based on title or timestamp.
// The extension is not included.
func (n *Note) GenerateFilename() string {
	if n.Title != "" {
		return SanitizeFilename(n.Title)
	}

	// Use timestamp
	t := n.Created
	if t.IsZero() {
		t = time.Now()
	}
	return t.Format("2006-01-02T15-04-05")
}

// SanitizeFilename removes or replaces invalid filename characters.
func SanitizeFilename(name string) string {
	// Replace characters that are invalid in filenames
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

	// Trim whitespace and dots from ends
	name = strings.TrimSpace(name)
	name = strings.Trim(name, ".")

	// Limit length
	if len(name) > 200 {
		name = name[:200]
	}

	return name
}

// ContentWithoutTitle returns the content with the title header line removed.
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

	// Trim leading empty lines
	for len(result) > 0 && strings.TrimSpace(result[0]) == "" {
		result = result[1:]
	}

	return strings.Join(result, "\n")
}
