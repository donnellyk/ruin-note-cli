package note

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const frontmatterDelimiter = "---"

type Frontmatter struct {
	UUID          string   `yaml:"uuid,omitempty"`
	Created       string   `yaml:"created,omitempty"`
	Updated       string   `yaml:"updated,omitempty"`
	Tags          []string `yaml:"tags,omitempty"`
	InlineTags    []string `yaml:"inline-tags,omitempty"`
	InheritedTags []string `yaml:"inherited-tags,omitempty"`
	Dates         []string `yaml:"dates,omitempty"`
	Parent        string   `yaml:"parent,omitempty"`
	Order         *int     `yaml:"order,omitempty"`
	LinkedCards   []string `yaml:"linked-cards,omitempty"`
	URL           string   `yaml:"url,omitempty"`

	Extra map[string]any `yaml:"-"`

	// originalNode is the parsed YAML mapping when frontmatter came from disk.
	// Nil for frontmatters constructed in code. Mutated in place by Serialize
	// so re-saved foreign frontmatter preserves comments, key order, and
	// scalar quote styles for keys ruin doesn't manage.
	originalNode *yaml.Node

	// originalExtra snapshots Extra at parse time. Serialize compares against
	// this to detect user-modified Extra keys and only re-encode those,
	// preserving the original scalar style on unchanged keys.
	originalExtra map[string]any
}

// typedFieldNames lists every YAML key managed as a typed field on Frontmatter.
// Used to decide what stays in Extra and what Serialize manages directly.
var typedFieldNames = map[string]bool{
	"uuid":           true,
	"created":        true,
	"updated":        true,
	"tags":           true,
	"inline-tags":    true,
	"inherited-tags": true,
	"dates":          true,
	"parent":         true,
	"order":          true,
	"linked-cards":   true,
	"url":            true,
}

// ParseFrontmatter extracts frontmatter from markdown content. If no
// frontmatter is present, returns an empty Frontmatter and the original content.
func ParseFrontmatter(content string) (*Frontmatter, string, error) {
	content = strings.TrimLeft(content, "\n\r")

	if !strings.HasPrefix(content, frontmatterDelimiter) {
		return &Frontmatter{Extra: make(map[string]any)}, content, nil
	}

	rest := content[len(frontmatterDelimiter):]
	before, after, ok := strings.Cut(rest, "\n"+frontmatterDelimiter)
	if !ok {
		return &Frontmatter{Extra: make(map[string]any)}, content, nil
	}

	fmYAML := before

	afterClosing := after
	if strings.HasPrefix(afterClosing, "\n") {
		afterClosing = afterClosing[1:]
	} else if strings.HasPrefix(afterClosing, "\r\n") {
		afterClosing = afterClosing[2:]
	}

	fm, err := parseFrontmatterYAML(fmYAML)
	if err != nil {
		return nil, "", err
	}

	return fm, afterClosing, nil
}

func parseFrontmatterYAML(yamlContent string) (*Frontmatter, error) {
	fm := &Frontmatter{Extra: make(map[string]any)}

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &root); err != nil {
		return nil, err
	}

	// Empty document (whitespace-only frontmatter) is valid; treat as empty.
	if len(root.Content) == 0 {
		return fm, nil
	}

	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: top-level frontmatter must be a YAML mapping", ErrInvalidFrontmatter)
	}

	fm.originalNode = mapping

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		valueNode := mapping.Content[i+1]
		key := keyNode.Value

		switch key {
		case "uuid":
			fm.UUID = valueNode.Value
		case "created":
			fm.Created = valueNode.Value
		case "updated":
			fm.Updated = valueNode.Value
		case "tags":
			fm.Tags = normalizeTagSlice(decodeStringSlice(valueNode))
		case "inline-tags":
			fm.InlineTags = normalizeTagSlice(decodeStringSlice(valueNode))
		case "inherited-tags":
			fm.InheritedTags = normalizeTagSlice(decodeStringSlice(valueNode))
		case "dates":
			fm.Dates = decodeStringSlice(valueNode)
		case "parent":
			fm.Parent = valueNode.Value
		case "order":
			if n, err := strconv.Atoi(valueNode.Value); err == nil {
				fm.Order = &n
			}
		case "linked-cards":
			fm.LinkedCards = decodeStringSlice(valueNode)
		case "url":
			fm.URL = valueNode.Value
		default:
			var v any
			if err := valueNode.Decode(&v); err == nil {
				fm.Extra[key] = v
			}
		}
	}

	fm.originalExtra = cloneAnyMap(fm.Extra)

	return fm, nil
}

// HasLegacyTagFrontmatter returns true if the file's frontmatter uses the
// pre-v0.4.0 tag format: any `#`-prefixed entry in `tags:` or
// `inherited-tags:`, or the presence of an `inline-tags:` key. Used by doctor
// to detect notes that need a tag-format migration rewrite.
//
// Operates on the raw file bytes since parseFrontmatterYAML strips the legacy
// `#` prefix on read (so the parsed Frontmatter no longer reveals the
// on-disk form).
func HasLegacyTagFrontmatter(content string) bool {
	content = strings.TrimLeft(content, "\n\r")
	if !strings.HasPrefix(content, frontmatterDelimiter) {
		return false
	}
	rest := content[len(frontmatterDelimiter):]
	before, _, ok := strings.Cut(rest, "\n"+frontmatterDelimiter)
	if !ok {
		return false
	}
	scanner := strings.Split(before, "\n")
	currentKey := ""
	for _, line := range scanner {
		stripped := strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(stripped)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "inline-tags:") {
			return true
		}
		isList := strings.HasPrefix(stripped, " ") || strings.HasPrefix(stripped, "\t")
		if !isList {
			currentKey = ""
			colon := strings.IndexByte(trimmed, ':')
			if colon <= 0 {
				continue
			}
			key := strings.TrimSpace(trimmed[:colon])
			value := strings.TrimSpace(trimmed[colon+1:])
			if key == "tags" || key == "inherited-tags" {
				if strings.HasPrefix(value, "[") {
					if strings.Contains(value, "#") {
						return true
					}
					continue
				}
				if value != "" {
					if strings.Contains(strings.Trim(value, `"' `), "#") {
						return true
					}
					continue
				}
				currentKey = key
			}
			continue
		}
		if currentKey != "" && strings.HasPrefix(trimmed, "- ") {
			val := strings.TrimSpace(trimmed[2:])
			val = strings.Trim(val, `"'`)
			if strings.HasPrefix(val, "#") {
				return true
			}
		}
	}
	return false
}

// normalizeTagSlice strips the `#` prefix/suffix and lowercases each entry,
// accepting both pre-v0.4.0 (`#tag`) and v0.4.0 (`tag`) on-disk forms.
func normalizeTagSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = NormalizeStored(s)
	}
	return out
}

func decodeStringSlice(n *yaml.Node) []string {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil
	}
	result := make([]string, 0, len(n.Content))
	for _, child := range n.Content {
		if child.Kind == yaml.ScalarNode {
			result = append(result, child.Value)
		}
	}
	return result
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	maps.Copy(out, in)
	return out
}

// Serialize converts the frontmatter to a YAML string with delimiters. Returns
// an empty string if the frontmatter is empty.
func (fm *Frontmatter) Serialize() (string, error) {
	if fm.IsEmpty() {
		return "", nil
	}

	if fm.originalNode != nil {
		return fm.serializeFromNode()
	}

	return fm.serializeFromMap()
}

// serializeFromMap is the legacy path used for Frontmatters constructed in code
// (no source YAML to preserve). Keys are sorted by yaml.v3's map encoding.
func (fm *Frontmatter) serializeFromMap() (string, error) {
	data := make(map[string]any)

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
	// inline-tags is never written: stored form lives in titles.json from v0.4.0.
	if len(fm.InheritedTags) > 0 {
		data["inherited-tags"] = fm.InheritedTags
	}
	if len(fm.Dates) > 0 {
		data["dates"] = fm.Dates
	}
	if fm.Parent != "" {
		data["parent"] = fm.Parent
	}
	if fm.Order != nil {
		data["order"] = *fm.Order
	}
	if len(fm.LinkedCards) > 0 {
		data["linked-cards"] = fm.LinkedCards
	}
	if fm.URL != "" {
		data["url"] = fm.URL
	}

	maps.Copy(data, fm.Extra)

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(data); err != nil {
		return "", err
	}
	encoder.Close()

	return frontmatterDelimiter + "\n" + buf.String() + frontmatterDelimiter + "\n", nil
}

// serializeFromNode mutates the parsed mapping in place to reflect typed-field
// and Extra changes, then marshals. Untouched keys keep their position,
// comments, and scalar style.
func (fm *Frontmatter) serializeFromNode() (string, error) {
	node := fm.originalNode

	setOrRemoveScalar(node, "uuid", fm.UUID)
	setOrRemoveScalar(node, "created", fm.Created)
	setOrRemoveScalar(node, "updated", fm.Updated)
	setOrRemoveStringSlice(node, "tags", fm.Tags)
	// inline-tags is never written: stored form lives in titles.json from v0.4.0.
	// Passing nil here strips the key from the source mapping.
	setOrRemoveStringSlice(node, "inline-tags", nil)
	setOrRemoveStringSlice(node, "inherited-tags", fm.InheritedTags)
	setOrRemoveStringSlice(node, "dates", fm.Dates)
	setOrRemoveScalar(node, "parent", fm.Parent)
	setOrRemoveOrder(node, fm.Order)
	setOrRemoveStringSlice(node, "linked-cards", fm.LinkedCards)
	setOrRemoveScalar(node, "url", fm.URL)

	syncExtraToNode(node, fm.Extra, fm.originalExtra)

	// Snapshot Extra for the next Serialize call so mutations between calls are detected.
	fm.originalExtra = cloneAnyMap(fm.Extra)

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		return "", err
	}
	encoder.Close()

	return frontmatterDelimiter + "\n" + buf.String() + frontmatterDelimiter + "\n", nil
}

// findKey returns the index in mapping.Content of the key node, or -1 if absent.
// Mapping content alternates [key, value]; the value lives at index+1.
func findKey(mapping *yaml.Node, key string) int {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// removeKeyAt splices out the key/value pair at the given key index.
func removeKeyAt(mapping *yaml.Node, idx int) {
	if idx < 0 || idx+1 >= len(mapping.Content) {
		return
	}
	mapping.Content = append(mapping.Content[:idx], mapping.Content[idx+2:]...)
}

func setOrRemoveScalar(mapping *yaml.Node, key, value string) {
	idx := findKey(mapping, key)
	if value == "" {
		if idx >= 0 {
			removeKeyAt(mapping, idx)
		}
		return
	}
	if idx >= 0 {
		valueNode := mapping.Content[idx+1]
		if valueNode.Value == value {
			return
		}
		valueNode.Kind = yaml.ScalarNode
		valueNode.Value = value
		valueNode.Tag = ""
		valueNode.Content = nil
		return
	}
	appendScalar(mapping, key, value)
}

func setOrRemoveOrder(mapping *yaml.Node, order *int) {
	idx := findKey(mapping, "order")
	if order == nil {
		if idx >= 0 {
			removeKeyAt(mapping, idx)
		}
		return
	}
	value := strconv.Itoa(*order)
	if idx >= 0 {
		valueNode := mapping.Content[idx+1]
		if valueNode.Value == value {
			return
		}
		valueNode.Kind = yaml.ScalarNode
		valueNode.Value = value
		valueNode.Tag = "!!int"
		valueNode.Style = 0
		valueNode.Content = nil
		return
	}
	appendInt(mapping, "order", value)
}

func setOrRemoveStringSlice(mapping *yaml.Node, key string, values []string) {
	idx := findKey(mapping, key)
	if len(values) == 0 {
		if idx >= 0 {
			removeKeyAt(mapping, idx)
		}
		return
	}
	if idx >= 0 {
		valueNode := mapping.Content[idx+1]
		if sequenceMatches(valueNode, values) {
			return
		}
		rebuildSequence(valueNode, values)
		return
	}
	appendSequence(mapping, key, values)
}

func sequenceMatches(n *yaml.Node, values []string) bool {
	if n.Kind != yaml.SequenceNode || len(n.Content) != len(values) {
		return false
	}
	for i, child := range n.Content {
		if child.Kind != yaml.ScalarNode || child.Value != values[i] {
			return false
		}
	}
	return true
}

func rebuildSequence(n *yaml.Node, values []string) {
	n.Kind = yaml.SequenceNode
	n.Tag = ""
	n.Style = 0
	n.Value = ""
	n.Content = make([]*yaml.Node, 0, len(values))
	for _, v := range values {
		n.Content = append(n.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Style: yaml.DoubleQuotedStyle,
			Value: v,
		})
	}
}

func appendScalar(mapping *yaml.Node, key, value string) {
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
}

func appendInt(mapping *yaml.Node, key, value string) {
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: value},
	)
}

func appendSequence(mapping *yaml.Node, key string, values []string) {
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, v := range values {
		seq.Content = append(seq.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Style: yaml.DoubleQuotedStyle,
			Value: v,
		})
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		seq,
	)
}

// syncExtraToNode reconciles the Node's non-typed keys with fm.Extra. Keys
// removed from Extra get spliced out of the Node; keys whose values changed get
// re-encoded; keys whose values are unchanged are left alone (preserving scalar
// style); new Extra keys are appended at the end of the mapping.
func syncExtraToNode(mapping *yaml.Node, current, original map[string]any) {
	// Remove or update existing non-typed keys.
	i := 0
	for i+1 < len(mapping.Content) {
		key := mapping.Content[i].Value
		if typedFieldNames[key] {
			i += 2
			continue
		}
		newVal, exists := current[key]
		if !exists {
			removeKeyAt(mapping, i)
			continue
		}
		if !reflect.DeepEqual(newVal, original[key]) {
			encoded, err := encodeAnyToNode(newVal)
			if err == nil {
				mapping.Content[i+1] = encoded
			}
		}
		i += 2
	}

	// Append keys that are new in Extra (not present in the Node).
	for key, val := range current {
		if typedFieldNames[key] {
			continue
		}
		if findKey(mapping, key) >= 0 {
			continue
		}
		encoded, err := encodeAnyToNode(val)
		if err != nil {
			continue
		}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			encoded,
		)
	}
}

// encodeAnyToNode encodes an arbitrary Go value through yaml.Marshal+Unmarshal
// to obtain a fully-populated yaml.Node we can splice into a mapping.
func encodeAnyToNode(v any) (*yaml.Node, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if len(root.Content) == 0 {
		return nil, fmt.Errorf("no content in encoded node")
	}
	return root.Content[0], nil
}

func (fm *Frontmatter) IsEmpty() bool {
	return fm.UUID == "" &&
		fm.Created == "" &&
		fm.Updated == "" &&
		len(fm.Tags) == 0 &&
		len(fm.InlineTags) == 0 &&
		len(fm.InheritedTags) == 0 &&
		len(fm.Dates) == 0 &&
		fm.Parent == "" &&
		fm.Order == nil &&
		len(fm.LinkedCards) == 0 &&
		fm.URL == "" &&
		len(fm.Extra) == 0
}

// Merge combines another frontmatter into this one. The other frontmatter's
// values take precedence for non-empty fields.
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
	if len(other.InheritedTags) > 0 {
		fm.InheritedTags = other.InheritedTags
	}
	if len(other.Dates) > 0 {
		fm.Dates = other.Dates
	}
	if other.Parent != "" {
		fm.Parent = other.Parent
	}
	if other.Order != nil {
		fm.Order = other.Order
	}
	if len(other.LinkedCards) > 0 {
		fm.LinkedCards = other.LinkedCards
	}
	if other.URL != "" {
		fm.URL = other.URL
	}

	if fm.Extra == nil {
		fm.Extra = make(map[string]any)
	}
	maps.Copy(fm.Extra, other.Extra)
}

var ErrInvalidFrontmatter = errors.New("invalid frontmatter")
