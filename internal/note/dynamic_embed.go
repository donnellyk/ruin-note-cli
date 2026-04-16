package note

import (
	"regexp"
	"strings"
)

var dynamicEmbedPattern = regexp.MustCompile(`^\s*!\[\[(\w+):\s*(.+?)\]\]\s*$`)

// DynamicEmbedRef represents a dynamic embed like ![[search: #daily @today | limit=5]].
type DynamicEmbedRef struct {
	Type    string
	Query   string
	Options map[string]string
	Line    int
}

// FindDynamicEmbeds returns embeds of recognized types (search, pick, query, compose).
func FindDynamicEmbeds(content string) []DynamicEmbedRef {
	lines := strings.Split(content, "\n")
	var refs []DynamicEmbedRef
	for i, line := range lines {
		m := dynamicEmbedPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		embedType := strings.ToLower(m[1])
		switch embedType {
		case "search", "pick", "query", "compose":
		default:
			continue
		}

		raw := m[2]
		query, opts := splitQueryOptions(raw)

		refs = append(refs, DynamicEmbedRef{
			Type:    embedType,
			Query:   strings.TrimSpace(query),
			Options: opts,
			Line:    i,
		})
	}
	return refs
}

func splitQueryOptions(raw string) (string, map[string]string) {
	idx := strings.Index(raw, "|")
	if idx < 0 {
		return raw, nil
	}
	return raw[:idx], ParseDynamicOptions(raw[idx+1:])
}

// ParseDynamicOptions parses a comma-separated option string. Bare flags are
// treated as key=true.
func ParseDynamicOptions(raw string) map[string]string {
	opts := make(map[string]string)
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if idx := strings.Index(p, "="); idx > 0 {
			opts[strings.TrimSpace(p[:idx])] = strings.TrimSpace(p[idx+1:])
		} else {
			opts[p] = "true"
		}
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}
