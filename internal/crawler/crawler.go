package crawler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	allowedHost = "pkg.go.dev"
	maxPages = 200
)


type Page struct{
	URL string
	Content string
}

type Crawler struct {
	client *http.Client
	visited  map[string]bool
	maxDepth int
	pages []Page
}
type queueItem struct{
	URL string
	depth int
}

func NewCrawler(depth int) *Crawler{
	return &Crawler{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		visited: make(map[string]bool),
		maxDepth: depth,
	}
}


func (c *Crawler) Crawl(startURL string) {
	queue := []queueItem{{URL: startURL, depth: 0}}

	for len(queue) >0 && len(c.pages) < maxPages{
	item := queue[0]
	queue = queue[1:]

	if item.depth >= c.maxDepth{
		continue;
	}
	if c.visited[item.URL] || !isAllowed(item.URL){
		continue;
	}
	c.visited[item.URL] = true
	time.Sleep(200 * time.Millisecond)

	resp, err := c.client.Get(item.URL)
	if err !=nil{
		fmt.Println("Error fetching URL:", err)
		continue
	}

	if resp.StatusCode != http.StatusOK{
		resp.Body.Close()
		continue
	}

	links, content := extractLinks(resp.Body)
	resp.Body.Close()
	c.pages = append(c.pages, Page{URL: item.URL, Content: content})

	for _, Link := range links{
		if !c.visited[Link] && isAllowed(Link){
			queue =append(queue, queueItem{URL: Link, depth: item.depth + 1})
		}
	}
	}

}

func isAllowed(RawURL string) bool{
	parsed,err := url.Parse(RawURL)
	if err != nil {
		return false
	}
	return parsed.Host == allowedHost
}

func extractLinks(body io.Reader) ([] string, string){
	 var links []string
	 var textBuilder strings.Builder

	 doc, err :=html.Parse(body)
	 if err !=nil{
		return links, ""
	 }

	 var walk func(*html.Node)
	 walk= func (n *html.Node)  {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "nav", "footer", "head":
				return
			}
		}
		if n.Type == html.ElementNode && n.Data=="a"{
			for _,attr := range n.Attr{
				if attr.Key == "href"{
					link := attr.Val

					if strings.HasPrefix(link, "/"){
						link = "https://" + allowedHost + link
					}
					if strings.HasPrefix(link, "http"){
						links = append(links, link)

					}
				}
			}
		}
		if n.Type == html.TextNode{
			content := strings.TrimSpace(n.Data)
			if content != ""{
                textBuilder.WriteString(content)
				textBuilder.WriteString(" ")
			}
		}
		for child :=n.FirstChild; child !=nil; child = child.NextSibling{
			walk(child)
	 }
	 }
	 walk(doc)
	 return links, textBuilder.String()
}

func (c *Crawler) GetPages() []Page{
	return c.pages
}