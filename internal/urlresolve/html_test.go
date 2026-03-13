package urlresolve

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTMLResolver_BasicTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>My Page Title</title></head><body></body></html>`)
	}))
	defer srv.Close()

	resolver := NewHTMLResolver()
	meta, err := resolver.Resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Title != "My Page Title" {
		t.Errorf("expected title %q, got %q", "My Page Title", meta.Title)
	}
	if meta.URL != srv.URL {
		t.Errorf("expected URL %q, got %q", srv.URL, meta.URL)
	}
	if meta.ResolvedVia != "html" {
		t.Errorf("expected resolved_via %q, got %q", "html", meta.ResolvedVia)
	}
}

func TestHTMLResolver_OGDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head>
			<title>Test</title>
			<meta property="og:description" content="OG description here">
			<meta name="description" content="Regular description">
		</head><body></body></html>`)
	}))
	defer srv.Close()

	resolver := NewHTMLResolver()
	meta, err := resolver.Resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Summary != "OG description here" {
		t.Errorf("expected og:description %q, got %q", "OG description here", meta.Summary)
	}
}

func TestHTMLResolver_MetaDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head>
			<title>Test</title>
			<meta name="description" content="Fallback description">
		</head><body></body></html>`)
	}))
	defer srv.Close()

	resolver := NewHTMLResolver()
	meta, err := resolver.Resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Summary != "Fallback description" {
		t.Errorf("expected description %q, got %q", "Fallback description", meta.Summary)
	}
}

func TestHTMLResolver_HTMLEntityDecoding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Tom &amp; Jerry &lt;3&gt; &quot;friends&quot; &#39;forever&#39;</title></head></html>`)
	}))
	defer srv.Close()

	resolver := NewHTMLResolver()
	meta, err := resolver.Resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `Tom & Jerry <3> "friends" 'forever'`
	if meta.Title != expected {
		t.Errorf("expected title %q, got %q", expected, meta.Title)
	}
}

func TestHTMLResolver_TitleTruncation(t *testing.T) {
	longTitle := strings.Repeat("a", 250)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><head><title>%s</title></head></html>`, longTitle)
	}))
	defer srv.Close()

	resolver := NewHTMLResolver()
	meta, err := resolver.Resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len([]rune(meta.Title)) != 200 {
		t.Errorf("expected title length 200 runes, got %d", len([]rune(meta.Title)))
	}
}

func TestHTMLResolver_DescriptionTruncation(t *testing.T) {
	longDesc := strings.Repeat("b", 600)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><head><meta name="description" content="%s"></head></html>`, longDesc)
	}))
	defer srv.Close()

	resolver := NewHTMLResolver()
	meta, err := resolver.Resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len([]rune(meta.Summary)) != 500 {
		t.Errorf("expected summary length 500 runes, got %d", len([]rune(meta.Summary)))
	}
}

func TestHTMLResolver_Non200StatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	resolver := NewHTMLResolver()
	_, err := resolver.Resolve(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for non-200 status code")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to contain status code 404, got %q", err.Error())
	}
}

func TestHTMLResolver_MissingTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head></head><body><p>No title here</p></body></html>`)
	}))
	defer srv.Close()

	resolver := NewHTMLResolver()
	meta, err := resolver.Resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Title != "" {
		t.Errorf("expected empty title, got %q", meta.Title)
	}
}

func TestHTMLResolver_LargeBodyLimited(t *testing.T) {
	// Serve a body larger than 1MB. The title is placed after the 1MB mark
	// so it should not be found due to the read limit.
	padding := strings.Repeat("x", 1<<20+100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><head>%s<title>Hidden Title</title></head></html>`, padding)
	}))
	defer srv.Close()

	resolver := NewHTMLResolver()
	meta, err := resolver.Resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Title appears after the 1MB limit, so it should not be extracted
	if meta.Title != "" {
		t.Errorf("expected empty title due to 1MB limit, got %q", meta.Title)
	}
}

func TestHTMLResolver_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until the request context is done
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	resolver := NewHTMLResolver()
	_, err := resolver.Resolve(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
