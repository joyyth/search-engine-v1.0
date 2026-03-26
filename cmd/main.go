package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joyyth/seacrh-engine-v1.0/internal/api"
	"github.com/joyyth/seacrh-engine-v1.0/internal/crawler"
	"github.com/joyyth/seacrh-engine-v1.0/internal/indexer"
	"github.com/joyyth/seacrh-engine-v1.0/internal/model"
	"github.com/joyyth/seacrh-engine-v1.0/internal/search"
	"github.com/joyyth/seacrh-engine-v1.0/internal/store"
)

const (
	docsPath  = "data/pages.json"
    indexPath = "data/index.json"
	startURL = "https://pkg.go.dev/std"
)

func main() {
	crawlMode := flag.Bool("crawl", false, "crawl pkg.go.dev/std")
	indexMode := flag.Bool("index", false, "index the crawled pages")
	serveMode := flag.Bool("serve", false, "serve the search engine")
	flag.Parse()

	if !*crawlMode && !*indexMode && !*serveMode {
        flag.Usage()
        os.Exit(1)
    }

	jsonStore := store.NewJSONStore(docsPath, indexPath)

	if *crawlMode{
		runCrawl(jsonStore)
	}
	if *indexMode{
		runIndex(jsonStore)
	}
	if *serveMode{
		runServe(jsonStore,":8082")
	}
}

func runCrawl(store store.Store){
	crawler := crawler.NewCrawler(3)
	crawler.Crawl(startURL)

	pages :=crawler.GetPages()
	docs := make([]model.Document, len(pages))
	for _, p := range pages {
        docs = append(docs, model.Document{
            URL:     p.URL,
            Title:   p.Title,
            Content: p.Content,
        })
    }
	if err :=store.SaveDocuments(docs); err !=nil{
		log.Fatalf("Failed to save documents: %v", err)
	}
	log.Printf("crawled %d pages, saved to disk", len(pages))
}

func runIndex(store store.Store){
	indexer := indexer.NewIndexer(store)

	docs, err := store.LoadDocuments()
	if err !=nil{
		log.Fatalf("Failed to load documents: %v", err)
	}

	for _,doc := range docs{
		indexer.AddDocument(doc)
	}

	if err:=indexer.Save(); err!=nil{
		log.Fatalf("Failed to save index: %v", err)
	}
	log.Printf("indexed %d documents, saved to disk", len(docs))
}

func runServe(s store.Store, addr string) {
    idx := indexer.NewIndexer(s)
    if err := idx.Load(); err != nil {
        log.Fatalf("failed to load index: %v", err)
    }

    srv := api.NewServer(search.NewSearch(idx))

    httpServer := srv.Start(addr) // expose the *http.Server from api package

    go func() {
        quit := make(chan os.Signal, 1)
        signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
        <-quit
        log.Println("shutting down...")
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        httpServer.Shutdown(ctx)
    }()

    log.Printf("starting server on %s", addr)
    if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
        log.Fatalf("server error: %v", err)
    }
}