package crawler

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	mu sync.Mutex
}

type queueItem struct {
	URL   string
	depth int
}

type workerResult struct{
	item queueItem
	doc model.Document
	links []string
	ok bool
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


func (c *Crawler) Crawl(startURL string, numWorkers int) {
        if numWorkers <= 0{
			numWorkers = 1
		}
		normalizedURL, err := utils.NormalizeURL(startURL)
		if err != nil {
			log.Printf("Error normalizing URL: %v", err)
			return
		}
		

        jobs := make(chan queueItem, maxPages)
		results := make(chan workerResult, maxPages)


		limiter := time.NewTicker(200*time.Millisecond)
		defer limiter.Stop()

		var workerWG sync.WaitGroup
		for i:=0; i<numWorkers;i++{
			workerWG.Add(1)
			go func(){
				defer workerWG.Done()
				c.worker(jobs,results,limiter.C)
			}()
		}
		go func(){
			workerWG.Wait()
			close(results)
		}()

		visited := map[string]bool{
			normalizedURL: true,
		}
		pending :=1
		jobs <- queueItem{
			URL: normalizedURL,
			depth: 0,
		}
		for pending >0 {
			result := <- results
			pending--

			if !result.ok{
				if pending == 0{
					break
				}
				continue
			}

			if !c.addPage(result.doc){
				if pending == 0{
					break
				}
				break
			}
			if result.item.depth >= c.maxDepth{
				continue
			}
			if c.limitReached(){
				break
			}

			for _, link := range result.links {
				normalizedLink, err := utils.NormalizeURL(link)
				if err != nil {
					log.Printf("Error normalizing link: %v", err)
					continue
				}
			    if !isAllowed(normalizedLink) || visited[normalizedLink]{
					continue
				}
				if c.limitReached(){
					break
				}
				visited[normalizedLink]=true
				pending++
				jobs <- queueItem{
					URL: normalizedLink,
					depth: result.item.depth + 1,
				}
			}
			if pending == 0{
				break
			}

			
		}

	close(jobs)
	workerWG.Wait()

}

func (c *Crawler) worker(jobs chan queueItem, results chan workerResult, rate <-chan time.Time){
	for item := range jobs{
		results <- c.fetch(item,rate)
	}
}
func (c *Crawler) fetch(item queueItem, rate <-chan time.Time) workerResult{
	if (item.depth > c.maxDepth || !isAllowed(item.URL)){
		return workerResult{
			item: item,
		}
	}
	<-rate
	
	resp, err := c.client.Get(item.URL)
	if err != nil {
		log.Printf("Error fetching URL: %v", err)
		return workerResult{
			item: item,
		}
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return workerResult{
			item: item,
		}
	}

	links, title, content := extractLinks(resp.Body)
	resp.Body.Close()
	return workerResult{
		item: item,
		doc: model.Document{
			URL: item.URL,
			Title: title,
			Content: content,
		},
		links: links,
		ok: true,
	}

}

func (c *Crawler) addPage(doc model.Document) bool{
	c.mu.Lock()
	 defer c.mu.Unlock()

	 if len(c.pages) >= maxPages{
		return false
	 }
	 c.pages=append(c.pages, doc)
	 return true
}

func (c *Crawler) limitReached() bool{
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pages) >= maxPages
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
