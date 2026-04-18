package crawler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joyyth/search-engine-v1.0/internal/model"
	"github.com/joyyth/search-engine-v1.0/internal/utils"
	"golang.org/x/net/html"
)

const (
	defaultAllowedHost = "en.wikipedia.org"
	defaultMaxPages    = 200
	defaultUserAgent   = "SearchEngineV1/0.1 (https://joyyth.com/bot; joyyth@gmail.com)"
)

// Config limits the crawl and sets the HTTP User-Agent. Zero fields use defaults from NewCrawler.
// HTTPClient, when set, is used instead of the default client (e.g. httptest.NewTLSServer's Client).
type Config struct {
	AllowedHost string
	MaxPages    int
	UserAgent   string
	HTTPClient  *http.Client
}

type Crawler struct {
	client   *http.Client
	cfg      Config
	visited  map[string]bool
	maxDepth int
	pages    []model.Document
	mu       sync.Mutex
	stats    crawlStats
}

type queueItem struct {
	URL   string
	depth int
}

type workerResult struct {
	item  queueItem
	doc   model.Document
	links []string
	ok    bool
}

type crawlStats struct {
	started       atomic.Int32
	succeeded     atomic.Int32
	requestErrors atomic.Int32
	nonOK         atomic.Int32
}

func NewCrawler(depth int, cfg Config) *Crawler {
	if cfg.AllowedHost == "" {
		cfg.AllowedHost = defaultAllowedHost
	}
	if cfg.MaxPages == 0 {
		cfg.MaxPages = defaultMaxPages
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}

	client := cfg.HTTPClient
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConns = 100
		transport.MaxIdleConnsPerHost = 100
		transport.MaxConnsPerHost = 100
		client = &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		}
	}

	return &Crawler{
		client:   client,
		cfg:      cfg,
		maxDepth: depth,
	}
}

func (c *Crawler) normalizeCrawlURL(rawURL string) (string, error) {
	normalizedURL, err := utils.NormalizeURL(rawURL)
	if err != nil {
		return "", err
	}

	parsed, err := url.Parse(normalizedURL)
	if err != nil {
		return "", err
	}

	if parsed.Host == c.cfg.AllowedHost {
		parsed.Scheme = "https"
		parsed.RawQuery = ""
	}

	return parsed.String(), nil
}

func (c *Crawler) Crawl(ctx context.Context, startURL string, numWorkers int) {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	crawlStart := time.Now()

	normalizedURL, err := c.normalizeCrawlURL(startURL)
	if err != nil {
		slog.Info("normalize start URL failed", "url", startURL, "error", err)
		return
	}
	if !c.isAllowed(normalizedURL) {
		slog.Info("start URL is not allowed", "url", startURL)
		return
	}

	select {
	case <-ctx.Done():
		return
	default:
	}

	jobs := make(chan queueItem, c.cfg.MaxPages)
	results := make(chan workerResult, c.cfg.MaxPages)

	var workerWG sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			c.worker(ctx, jobs, results)
		}()
	}
	go func() {
		workerWG.Wait()
		close(results)
	}()

	c.visited = map[string]bool{normalizedURL: true}
	queue := []queueItem{{URL: normalizedURL, depth: 0}}
	pending := 0

	handleResult := func(result workerResult) {
		if !result.ok {
			return
		}
		if !c.tryAddPage(result.doc) {
			return
		}
		if result.item.depth >= c.maxDepth {
			return
		}
		for _, link := range result.links {
			normalizedLink, err := c.normalizeCrawlURL(link)
			if err != nil || !c.isAllowed(normalizedLink) || c.visited[normalizedLink] {
				continue
			}
			if c.pageCount() >= c.cfg.MaxPages {
				break
			}
			c.visited[normalizedLink] = true
			queue = append(queue, queueItem{URL: normalizedLink, depth: result.item.depth + 1})
		}
	}

	for len(queue) > 0 || pending > 0 {
		if c.pageCount() >= c.cfg.MaxPages {
			queue = nil
		}

	drainResults:
		for pending > 0 {
			select {
			case <-ctx.Done():
				queue = nil
				break drainResults
			case result := <-results:
				pending--
				handleResult(result)
			default:
				break drainResults
			}
		}

		if len(queue) == 0 {
			if pending == 0 {
				break
			}
			select {
			case <-ctx.Done():
				queue = nil
			case result := <-results:
				pending--
				handleResult(result)
			}
			continue
		}

		select {
		case <-ctx.Done():
			queue = nil
		case result := <-results:
			pending--
			handleResult(result)
		case jobs <- queue[0]:
			queue = queue[1:]
			pending++
		}
	}

	close(jobs)
	workerWG.Wait()

	slog.Info("crawl complete",
		"duration", time.Since(crawlStart),
		"workers", numWorkers,
		"pages", c.pageCount(),
		"started_fetches", c.stats.started.Load(),
		"succeeded_fetches", c.stats.succeeded.Load(),
		"request_errors", c.stats.requestErrors.Load(),
		"non_ok_responses", c.stats.nonOK.Load(),
	)
}

func (c *Crawler) worker(ctx context.Context, jobs chan queueItem, results chan workerResult) {
	for item := range jobs {
		results <- c.fetch(ctx, item)
	}
}

func (c *Crawler) fetch(ctx context.Context, item queueItem) workerResult {
	start := time.Now()
	c.stats.started.Add(1)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
	if err != nil {
		c.stats.requestErrors.Add(1)
		return workerResult{item: item}
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.client.Do(req)
	if err != nil {
		c.stats.requestErrors.Add(1)
		slog.Info("fetch failed",
			"url", item.URL,
			"depth", item.depth,
			"duration", time.Since(start),
			"error", err,
		)
		return workerResult{
			item: item,
		}
	}

	if resp.StatusCode != http.StatusOK {
		c.stats.nonOK.Add(1)
		slog.Info("fetch skipped",
			"url", item.URL,
			"depth", item.depth,
			"status", resp.StatusCode,
			"duration", time.Since(start),
		)
		resp.Body.Close()
		return workerResult{
			item: item,
		}
	}

	links, title, content := extractLinks(resp.Body, c.cfg.AllowedHost)
	resp.Body.Close()
	c.stats.succeeded.Add(1)
	slog.Info("fetch complete",
		"url", item.URL,
		"depth", item.depth,
		"status", resp.StatusCode,
		"links", len(links),
		"duration", time.Since(start),
	)
	return workerResult{
		item: item,
		doc: model.Document{
			URL:     item.URL,
			Title:   title,
			Content: content,
		},
		links: links,
		ok:    true,
	}
}

func (c *Crawler) pageCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pages)
}

func (c *Crawler) tryAddPage(doc model.Document) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.pages) >= c.cfg.MaxPages {
		return false
	}
	c.pages = append(c.pages, doc)
	return true
}

func (c *Crawler) isAllowed(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.Host == c.cfg.AllowedHost
}

func extractLinks(body io.Reader, allowedHost string) ([]string, string, string) {
	var links []string
	var textBuilder strings.Builder
	var title string

	doc, err := html.Parse(body)
	if err != nil {
		return links, "", ""
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {

		if n.Type == html.ElementNode && n.Data == "title" {
			if child := n.FirstChild; child != nil && child.Type == html.TextNode {
				title = strings.TrimSpace(child.Data)
			}
			return
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "nav", "footer":
				return
			}
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					link := attr.Val

					switch {
					case strings.HasPrefix(link, "//"):
						link = "https:" + link
					case strings.HasPrefix(link, "/"):
						link = "https://" + allowedHost + link
					case !strings.HasPrefix(link, "http"):
						continue
					}
					links = append(links, link)
				}
			}
		}
		if n.Type == html.TextNode {
			content := strings.TrimSpace(n.Data)
			if content != "" {
				textBuilder.WriteString(content)
				textBuilder.WriteString(" ")
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return links, title, textBuilder.String()
}

func (c *Crawler) GetPages() []model.Document {
	c.mu.Lock()
	defer c.mu.Unlock()
	pages := make([]model.Document, len(c.pages))
	copy(pages, c.pages)
	return pages
}
