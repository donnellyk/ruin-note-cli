package note

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const frontmatterDelimiter = "---"

// Frontmatter represents the YAML frontmatter of a note.
type Frontmatter struct {
	UUID       string   `yaml:"uuid,omitempty"`
	Created    string   `yaml:"created,omitempty"`
	Updated    string   `yaml:"updated,omitempty"`
	Tags       []string `yaml:"tags,omitempty"`
	InlineTags []string `yaml:"inline-tags,omitempty"`

	// Extra holds any additional frontmatter fields not explicitly defined.
	// This preserves user-added fields.
	Extra map[string]interface{} `yaml:"-"`
}

// ParseFrontmatter extracts frontmatter from markdown content.
// Returns the frontmatter, the remaining content (without frontmatter), and any error.
// If no frontmatter is present, returns an empty Frontmatter and the original content.
func ParseFrontmatter(content string) (*Frontmatter, string, error) {
	content = strings.TrimLeft(content, "\n\r")

	// Check if content starts with frontmatter delimiter
	if !strings.HasPrefix(content, frontmatterDelimiter) {
		return &Frontmatter{Extra: make(map[string]interface{})}, content, nil
	}

	// Find the closing delimiter
	rest := content[len(frontmatterDelimiter):]
	closingIdx := strings.Index(rest, "\n"+frontmatterDelimiter)
	if closingIdx == -1 {
		// No closing delimiter, treat as no frontmatter
		return &Frontmatter{Extra: make(map[string]interface{})}, content, nil
	}

	// Extract frontmatter YAML
	fmYAML := rest[:closingIdx]

	// Find where content starts after closing delimiter
	afterClosing := rest[closingIdx+len("\n"+frontmatterDelimiter):]
	// Skip the newline after closing delimiter if present
	if strings.HasPrefix(afterClosing, "\n") {
		afterClosing = afterClosing[1:]
	} else if strings.HasPrefix(afterClosing, "\r\n") {
		afterClosing = afterClosing[2:]
	}

	// Parse frontmatter
	fm, err := parseFrontmatterYAML(fmYAML)
	if err != nil {
		return nil, "", err
	}

	return fm, afterClosing, nil
}

func parseFrontmatterYAML(yamlContent string) (*Frontmatter, error) {
	fm := &Frontmatter{Extra: make(map[string]interface{})}

	// First, unmarshal into a generic map to capture all fields
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &raw); err != nil {
		return nil, err
	}

	// Extract known fields
	if v, ok := raw["uuid"].(string); ok {
		fm.UUID = v
		delete(raw, "uuid")
	}
	if v := raw["created"]; v != nil {
		fm.Created = valueToString(v)
		delete(raw, "created")
	}
	if v := raw["updated"]; v != nil {
		fm.Updated = valueToString(v)
		delete(raw, "updated")
	}
	if v, ok := raw["tags"]; ok {
		fm.Tags = toStringSlice(v)
		delete(raw, "tags")
	}
	if v, ok := raw["inline-tags"]; ok {
		fm.InlineTags = toStringSlice(v)
		delete(raw, "inline-tags")
	}

	// Store remaining fields as extra
	fm.Extra = raw

	return fm, nil
}

// valueToString converts various types to string (handles time.Time from YAML)
func valueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case time.Time:
		return val.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return val
	default:
		return nil
	}
}

// Serialize converts the frontmatter to a YAML string with delimiters.
// Returns an empty string if the frontmatter is empty.
func (fm *Frontmatter) Serialize() (string, error) {
	if fm.IsEmpty() {
		return "", nil
	}

	// Build a map with known fields first (for ordering), then extra
	data := make(map[string]interface{})

	// Add known fields in preferred order
	if fm.UUID != "" {
		data["uuid"] = fm.UUID
	}
	if fm.Created != "" {
		data["created"] = fm.Created
	}
	if fm.Updated != "" {
		data["updated"] = fm.Updated
	}
	if len(fm.Tags) > 0 {
		data["tags"] = fm.Tags
	}
	if len(fm.InlineTags) > 0 {
		data["inline-tags"] = fm.InlineTags
	}

	// Add extra fields
	for k, v := range fm.Extra {
		data[k] = v
	}

	// Use yaml.v3 encoder for better control
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(data); err != nil {
		return "", err
	}
	encoder.Close()

	return frontmatterDelimiter + "\n" + buf.String() + frontmatterDelimiter + "\n", nil
}

// IsEmpty returns true if the frontmatter has no data.
func (fm *Frontmatter) IsEmpty() bool {
	return fm.UUID == "" &&
		fm.Created == "" &&
		fm.Updated == "" &&
		len(fm.Tags) == 0 &&
		len(fm.InlineTags) == 0 &&
		len(fm.Extra) == 0
}

// Merge combines another frontmatter into this one.
// The other frontmatter's values take precedence for non-empty fields.
func (fm *Frontmatter) Merge(other *Frontmatter) {
	if other == nil {
		return
	}

	if other.UUID != "" {
		fm.UUID = other.UUID
	}
	if other.Created != "" {
		fm.Created = other.Created
	}
	if other.Updated != "" {
		fm.Updated = other.Updated
	}
	if len(other.Tags) > 0 {
		fm.Tags = other.Tags
	}
	if len(other.InlineTags) > 0 {
		fm.InlineTags = other.InlineTags
	}

	// Merge extra fields
	if fm.Extra == nil {
		fm.Extra = make(map[string]interface{})
	}
	for k, v := range other.Extra {
		fm.Extra[k] = v
	}
}

// ErrInvalidFrontmatter indicates malformed frontmatter.
var ErrInvalidFrontmatter = errors.New("invalid frontmatter")
