package crawler

import (
	"io"
	"log"
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
	allowedHost = "en.wikipedia.org"
	maxPages    = 200
	userAgent   = "SearchEngineV1/0.1 (https://joyyth.com/bot; joyyth@gmail.com)"
)

type Crawler struct {
	client   *http.Client
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

type crawlStatsSnapshot struct {
	started       int32
	succeeded     int32
	requestErrors int32
	nonOK         int32
}

func NewCrawler(depth int) *Crawler {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 100
	transport.MaxConnsPerHost = 100

	return &Crawler{
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
		visited:  make(map[string]bool),
		maxDepth: depth,
	}
}

func normalizeCrawlURL(rawURL string) (string, error) {
	normalizedURL, err := utils.NormalizeURL(rawURL)
	if err != nil {
		return "", err
	}

	parsed, err := url.Parse(normalizedURL)
	if err != nil {
		return "", err
	}

	if parsed.Host == allowedHost {
		parsed.Scheme = "https"
		parsed.RawQuery = ""
	}

	return parsed.String(), nil
}

func (c *Crawler) Crawl(startURL string, numWorkers int) {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	crawlStart := time.Now()

	normalizedURL, err := normalizeCrawlURL(startURL)
	if err != nil {
		log.Printf("Error normalizing URL: %v", err)
		return
	}

	jobs := make(chan queueItem, maxPages)
	results := make(chan workerResult, maxPages)

	var workerWG sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			c.worker(jobs, results)
		}()
	}
	go func() {
		workerWG.Wait()
		close(results)
	}()

	visited := map[string]bool{normalizedURL: true}
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
			normalizedLink, err := normalizeCrawlURL(link)
			if err != nil || !isAllowed(normalizedLink) || visited[normalizedLink] {
				continue
			}
			if c.pageCount() >= maxPages {
				break
			}
			visited[normalizedLink] = true
			queue = append(queue, queueItem{URL: normalizedLink, depth: result.item.depth + 1})
		}
	}

	for len(queue) > 0 || pending > 0 {
		if c.pageCount() >= maxPages {
			queue = nil
		}

	drainResults:
		for pending > 0 {
			select {
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
			result := <-results
			pending--
			handleResult(result)
			continue
		}

		select {
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

	stats := c.statsSnapshot()
	slog.Info("crawl complete",
		"duration", time.Since(crawlStart),
		"workers", numWorkers,
		"pages", c.pageCount(),
		"started_fetches", stats.started,
		"succeeded_fetches", stats.succeeded,
		"request_errors", stats.requestErrors,
		"non_ok_responses", stats.nonOK,
	)
}

func (c *Crawler) worker(jobs chan queueItem, results chan workerResult) {
	for item := range jobs {
		results <- c.fetch(item)
	}
}
func (c *Crawler) fetch(item queueItem) workerResult {
	if item.depth > c.maxDepth || !isAllowed(item.URL) {
		return workerResult{
			item: item,
		}
	}

	start := time.Now()
	c.recordFetchStart()

	req, err := http.NewRequest(http.MethodGet, item.URL, nil)
	if err != nil {
		c.recordRequestError()
		return workerResult{item: item}
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.client.Do(req)
	if err != nil {
		c.recordRequestError()
		slog.Info("fetch failed",
			"url", item.URL,
			"depth", item.depth,
			"duration", time.Since(start),
			"error", err,
		)
		log.Printf("Error fetching URL: %v", err)
		return workerResult{
			item: item,
		}
	}

	if resp.StatusCode != http.StatusOK {
		c.recordNonOK()
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

	links, title, content := extractLinks(resp.Body)
	resp.Body.Close()
	c.recordFetchSuccess()
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
	if len(c.pages) >= maxPages {
		return false
	}
	c.pages = append(c.pages, doc)
	return true
}

func (c *Crawler) recordFetchStart() {
	c.stats.started.Add(1)
}

func (c *Crawler) recordFetchSuccess() {
	c.stats.succeeded.Add(1)
}

func (c *Crawler) recordRequestError() {
	c.stats.requestErrors.Add(1)
}

func (c *Crawler) recordNonOK() {
	c.stats.nonOK.Add(1)
}

func (c *Crawler) statsSnapshot() crawlStatsSnapshot {
	return crawlStatsSnapshot{
		started:       c.stats.started.Load(),
		succeeded:     c.stats.succeeded.Load(),
		requestErrors: c.stats.requestErrors.Load(),
		nonOK:         c.stats.nonOK.Load(),
	}
}

func isAllowed(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.Host == allowedHost
}

func extractLinks(body io.Reader) ([]string, string, string) {
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

					if strings.HasPrefix(link, "/") {
						link = "https://" + allowedHost + link
					}
					if strings.HasPrefix(link, "http") {
						links = append(links, link)

					}
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
