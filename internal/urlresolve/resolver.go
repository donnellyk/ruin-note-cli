package urlresolve

import "context"

type URLMetadata struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Summary     string `json:"summary,omitempty"`
	ResolvedVia string `json:"resolved_via"`
}

type Resolver interface {
	Resolve(ctx context.Context, url string) (*URLMetadata, error)
}
