package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/joyyth/search-engine-v1.0/internal/model"
)

func TestNewCrawler_DefaultConfig(t *testing.T) {
	t.Parallel()
	c := NewCrawler(3, Config{})
	if c.cfg.AllowedHost != defaultAllowedHost {
		t.Fatalf("AllowedHost = %q, want %q", c.cfg.AllowedHost, defaultAllowedHost)
	}
	if c.cfg.MaxPages != defaultMaxPages {
		t.Fatalf("MaxPages = %d, want %d", c.cfg.MaxPages, defaultMaxPages)
	}
	if c.cfg.UserAgent != defaultUserAgent {
		t.Fatalf("UserAgent = %q, want %q", c.cfg.UserAgent, defaultUserAgent)
	}
	if c.maxDepth != 3 {
		t.Fatalf("maxDepth = %d, want 3", c.maxDepth)
	}
	if c.client == nil {
		t.Fatal("expected non-nil HTTP client")
	}
}

func TestNormalizeCrawlURL(t *testing.T) {
	t.Parallel()
	c := NewCrawler(1, Config{AllowedHost: "en.wikipedia.org"})

	got, err := c.normalizeCrawlURL("https://en.wikipedia.org/wiki/Foo?query=1#frag")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	want := "https://en.wikipedia.org/wiki/Foo"
	if got != want {
		t.Fatalf("allowed host URL = %q, want %q", got, want)
	}

	otherHost := NewCrawler(1, Config{AllowedHost: "example.com"})
	out, err := otherHost.normalizeCrawlURL("https://OTHER.example.com/path/")
	if err != nil {
		t.Fatalf("normalize other: %v", err)
	}
	if !strings.Contains(out, "other.example.com") {
		t.Fatalf("expected lowercase host in %q", out)
	}
}

func TestIsAllowed(t *testing.T) {
	t.Parallel()
	c := NewCrawler(0, Config{AllowedHost: "en.wikipedia.org"})
	if !c.isAllowed("https://en.wikipedia.org/wiki/Go") {
		t.Fatal("expected allowed")
	}
	if c.isAllowed("https://evil.example/wiki/Go") {
		t.Fatal("expected disallowed")
	}
	if c.isAllowed("://broken") {
		t.Fatal("expected disallowed for bad URL")
	}
}

func TestExtractLinks(t *testing.T) {
	t.Parallel()
	const host = "en.wikipedia.org"
	html := `<!doctype html>
<html><head><title>  Hello Title  </title></head>
<body>
<script>ignored</script>
<style>.x{}</style>
<nav><a href="/nav">skip nav tree</a></nav>
<a href="/wiki/A">relative</a>
<a href="https://en.wikipedia.org/wiki/B">absolute same host</a>
<a href="//en.wikipedia.org/wiki/C">protocol-relative</a>
<a href="mailto:x@y">mail link</a>
<a href="#frag">hash only</a>
<footer>foot</footer>
<p>Body text here.</p>
</body></html>`
	links, title, content := extractLinks(strings.NewReader(html), host)
	if title != "Hello Title" {
		t.Fatalf("title = %q", title)
	}
	if !strings.Contains(content, "Body text here") {
		t.Fatalf("content missing body text: %q", content)
	}
	if strings.Contains(content, "ignored") {
		t.Fatalf("script text should not appear in content")
	}

	wantSubs := []string{
		"https://" + host + "/wiki/A",
		"https://en.wikipedia.org/wiki/B",
		"https://en.wikipedia.org/wiki/C",
	}
	for _, w := range wantSubs {
		if !slices.Contains(links, w) {
			t.Fatalf("links %v missing %q", links, w)
		}
	}
}

func TestExtractLinks_ParseError(t *testing.T) {
	t.Parallel()
	links, title, content := extractLinks(strings.NewReader(""), "example.com")
	if len(links) != 0 || title != "" || content != "" {
		t.Fatalf("expected empty got links=%v title=%q content=%q", links, title, content)
	}
}

func tlsCrawlServer(t *testing.T, mux *http.ServeMux) (*httptest.Server, string) {
	t.Helper()
	ts := httptest.NewTLSServer(mux)
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return ts, u.Host
}

func TestCrawl_SinglePage_MaxDepthZero(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Root</title></head>
		<body><a href="/other">skip</a>hello</body></html>`)
	})
	ts, host := tlsCrawlServer(t, mux)

	cfg := Config{
		AllowedHost: host,
		MaxPages:    10,
		UserAgent:   "test-bot",
		HTTPClient:  ts.Client(),
	}
	c := NewCrawler(0, cfg)
	c.Crawl(context.Background(), ts.URL+"/start", 2)

	pages := c.GetPages()
	if len(pages) != 1 {
		t.Fatalf("len(pages) = %d, want 1", len(pages))
	}
	if pages[0].Title != "Root" {
		t.Fatalf("Title = %q", pages[0].Title)
	}
	if !strings.Contains(pages[0].Content, "hello") {
		t.Fatalf("Content = %q", pages[0].Content)
	}
	if pages[0].URL != ts.URL+"/start" {
		t.Fatalf("URL = %q", pages[0].URL)
	}
}

func TestCrawl_FollowsLinks_RespectsMaxPages(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><title>A</title></head><body>
		<a href="/b">b</a> <a href="/c">c</a> alpha</body></html>`)
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><title>B</title></head><body>beta</body></html>`)
	})
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><title>C</title></head><body>gamma</body></html>`)
	})
	ts, host := tlsCrawlServer(t, mux)

	cfg := Config{
		AllowedHost: host,
		MaxPages:    2,
		HTTPClient:  ts.Client(),
	}
	c := NewCrawler(2, cfg)
	c.Crawl(context.Background(), ts.URL+"/a", 3)

	pages := c.GetPages()
	if len(pages) != 2 {
		t.Fatalf("len(pages) = %d, want 2 (MaxPages)", len(pages))
	}
	titles := make(map[string]struct{})
	for _, p := range pages {
		titles[p.Title] = struct{}{}
	}
	if _, ok := titles["A"]; !ok {
		t.Fatalf("missing page A: %#v", pages)
	}
}

func TestCrawl_DisallowedStartHost(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>X</title></head><body>x</body></html>`)
	})
	ts, _ := tlsCrawlServer(t, mux)

	c := NewCrawler(1, Config{
		AllowedHost: "wrong.host.example",
		MaxPages:    5,
		HTTPClient:  ts.Client(),
	})
	c.Crawl(context.Background(), ts.URL+"/", 1)
	if len(c.GetPages()) != 0 {
		t.Fatalf("expected no pages, got %d", len(c.GetPages()))
	}
}

func TestCrawl_InvalidStartURL(t *testing.T) {
	t.Parallel()
	c := NewCrawler(1, Config{MaxPages: 5})
	c.Crawl(context.Background(), "not-a-valid-url", 1)
	if len(c.GetPages()) != 0 {
		t.Fatal("expected no pages")
	}
}

func TestCrawl_WorkersDefaultToOne(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/only", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><title>One</title></head><body>ok</body></html>`)
	})
	ts, host := tlsCrawlServer(t, mux)

	c := NewCrawler(0, Config{AllowedHost: host, MaxPages: 3, HTTPClient: ts.Client()})
	c.Crawl(context.Background(), ts.URL+"/only", 0)
	if len(c.GetPages()) != 1 {
		t.Fatalf("got %d pages", len(c.GetPages()))
	}
}

func TestCrawl_ContextCancelled(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		<-block
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><title>Slow</title></head><body></body></html>`)
	})
	ts, host := tlsCrawlServer(t, mux)

	cfg := Config{AllowedHost: host, MaxPages: 10, HTTPClient: ts.Client()}
	c := NewCrawler(3, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
		close(block)
	}()
	c.Crawl(ctx, ts.URL+"/slow", 1)
}

func TestGetPages_Copy(t *testing.T) {
	t.Parallel()
	c := NewCrawler(0, Config{MaxPages: 5})
	c.mu.Lock()
	c.pages = []model.Document{{Title: "t", URL: "u"}}
	c.mu.Unlock()
	p1 := c.GetPages()
	p2 := c.GetPages()
	p1[0].Title = "mutated"
	if c.pages[0].Title != "t" {
		t.Fatalf("internal pages mutated: %q", c.pages[0].Title)
	}
	if p2[0].Title != "t" {
		t.Fatalf("second GetPages saw mutation: %q", p2[0].Title)
	}
}
