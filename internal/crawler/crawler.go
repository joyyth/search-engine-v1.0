package crawler

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/joyyth/search-engine-v1.0/internal/model"
	"github.com/joyyth/search-engine-v1.0/internal/utils"
	"golang.org/x/net/html"
)

const (
	allowedHost = "pkg.go.dev"
	maxPages    = 200
)

type Crawler struct {
	client   *http.Client
	visited  map[string]bool
	maxDepth int
	pages    []model.Document
}
type queueItem struct {
	URL   string
	depth int
}

func NewCrawler(depth int) *Crawler {
	return &Crawler{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		visited:  make(map[string]bool),
		maxDepth: depth,
	}
}

func (c *Crawler) Crawl(startURL string) {
	normalizedURL, err := utils.NormalizeURL(startURL)
	if err != nil {
		log.Printf("Error normalizing URL: %v", err)
		return
	}
	head := 0 // track queue head index
	queue := []queueItem{{URL: normalizedURL, depth: 0}}

	for head < len(queue) && len(c.pages) < maxPages {
		item := queue[head]
		head++

		if item.depth >= c.maxDepth {
			continue
		}
		if c.visited[item.URL] || !isAllowed(item.URL) {
			continue
		}
		c.visited[item.URL] = true
		time.Sleep(200 * time.Millisecond)

		resp, err := c.client.Get(item.URL)
		if err != nil {
			log.Printf("Error fetching URL: %v", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		links, title, content := extractLinks(resp.Body)
		resp.Body.Close()
		c.pages = append(c.pages, model.Document{URL: item.URL, Title: title, Content: content})

		for _, link := range links {
			normalizedLink, err := utils.NormalizeURL(link)
			if err != nil {
				log.Printf("Error normalizing link: %v", err)
				continue
			}
			if !c.visited[normalizedLink] && isAllowed(normalizedLink) {
				queue = append(queue, queueItem{URL: normalizedLink, depth: item.depth + 1})
			}
		}
		// Periodically compact processed items so the backing array
		// does not retain too much already-consumed queue data.
		if head > 1024 && head*2 >= len(queue) {
			queue = append([]queueItem(nil), queue[head:]...)
			head = 0
		}

	}

}

func isAllowed(RawURL string) bool {
	parsed, err := url.Parse(RawURL)
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
			case "script", "style", "nav", "footer", "head":
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
	return c.pages
}
