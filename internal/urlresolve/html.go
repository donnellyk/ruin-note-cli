package urlresolve

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTMLResolver struct {
	Client *http.Client
}

func NewHTMLResolver() *HTMLResolver {
	return &HTMLResolver{
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (r *HTMLResolver) Resolve(ctx context.Context, rawURL string) (*URLMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "ruin-note-cli")

	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, 1<<20)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	html := string(body)
	meta := &URLMetadata{
		URL:         rawURL,
		ResolvedVia: "html",
	}

	meta.Title = extractTitle(html)
	meta.Summary = extractDescription(html)

	return meta, nil
}

func extractTitle(html string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title")
	if start == -1 {
		return ""
	}
	tagEnd := strings.IndexByte(lower[start:], '>')
	if tagEnd == -1 {
		return ""
	}
	contentStart := start + tagEnd + 1
	closeIdx := strings.Index(lower[contentStart:], "</title>")
	if closeIdx == -1 {
		return ""
	}
	title := html[contentStart : contentStart+closeIdx]
	title = decodeHTMLEntities(title)
	title = strings.TrimSpace(title)
	return truncateRunes(title, 200)
}

func extractDescription(html string) string {
	lower := strings.ToLower(html)

	// og:description is preferred over meta description when both are present.
	patterns := []string{
		`<meta property="og:description"`,
		`<meta name="description"`,
	}

	for _, pattern := range patterns {
		idx := strings.Index(lower, pattern)
		if idx == -1 {
			continue
		}
		tagEnd := strings.IndexByte(lower[idx:], '>')
		if tagEnd == -1 {
			continue
		}
		tag := html[idx : idx+tagEnd+1]
		content := extractMetaContent(tag)
		if content != "" {
			content = decodeHTMLEntities(content)
			return truncateRunes(strings.TrimSpace(content), 500)
		}
	}
	return ""
}

func extractMetaContent(tag string) string {
	lower := strings.ToLower(tag)
	idx := strings.Index(lower, "content=")
	if idx == -1 {
		return ""
	}
	rest := tag[idx+8:]
	if len(rest) == 0 {
		return ""
	}

	quote := rest[0]
	if quote != '"' && quote != '\'' {
		return ""
	}
	end := strings.IndexByte(rest[1:], quote)
	if end == -1 {
		return ""
	}
	return rest[1 : end+1]
}

func decodeHTMLEntities(s string) string {
	replacements := []struct{ old, new string }{
		{"&amp;", "&"},
		{"&lt;", "<"},
		{"&gt;", ">"},
		{"&quot;", "\""},
		{"&#39;", "'"},
		{"&apos;", "'"},
		{"&#x27;", "'"},
		{"&nbsp;", " "},
	}
	for _, r := range replacements {
		s = strings.ReplaceAll(s, r.old, r.new)
	}
	return s
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
