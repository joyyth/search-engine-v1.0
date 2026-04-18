package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joyyth/search-engine-v1.0/internal/api"
	"github.com/joyyth/search-engine-v1.0/internal/crawler"
	"github.com/joyyth/search-engine-v1.0/internal/indexer"
	"github.com/joyyth/search-engine-v1.0/internal/model"
	"github.com/joyyth/search-engine-v1.0/internal/search"
	"github.com/joyyth/search-engine-v1.0/internal/store"
)

const (
	docsPath  = "data/pages.json"
	indexPath = "data/index.json"
	startURL  = "https://pkg.go.dev/std"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	crawlMode := flag.Bool("crawl", false, "crawl pkg.go.dev/std")
	indexMode := flag.Bool("index", false, "index the crawled pages")
	serveMode := flag.Bool("serve", false, "serve the search engine")
	flag.Parse()

	if !*crawlMode && !*indexMode && !*serveMode {
		flag.Usage()
		os.Exit(1)
	}

	jsonStore, err := store.NewJSONStore(docsPath, indexPath)
	if err != nil {
		log.Fatalf("json store: %v", err)
	}

	if *crawlMode {
		runCrawl(jsonStore)
	}
	if *indexMode {
		runIndex(jsonStore)
	}
	if *serveMode {
		runServe(jsonStore, ":8082")
	}
}

func runCrawl(store store.Store) {
	start := time.Now()
	crawler := crawler.NewCrawler(3, crawler.Config{
		AllowedHost:       "pkg.go.dev",
		MaxPages:          200,
		UserAgent:         "SearchEngineV1/0.1 (https://joyyth.com/bot; joyyth@gmail.com)",
		RequestsPerSecond: 2,
	})
	crawler.Crawl(context.Background(), startURL, 4)
	crawlDuration := time.Since(start)
	pages := crawler.GetPages()
	docs := make([]model.Document, 0, len(pages))
	for _, p := range pages {
		docs = append(docs, model.Document{
			URL:     p.URL,
			Title:   p.Title,
			Content: p.Content,
		})
	}
	if err := store.SaveDocuments(docs); err != nil {
		log.Fatalf("Failed to save documents: %v", err)
	}
	slog.Info("crawl saved",
		"pages", len(pages),
		"crawl_duration", crawlDuration,
		"total_duration", time.Since(start),
	)
}

func runIndex(store store.Store) {
	indexer := indexer.NewIndexer(store)

	docs, err := store.LoadDocuments()
	if err != nil {
		log.Fatalf("Failed to load documents: %v", err)
	}

	for _, doc := range docs {
		indexer.AddDocument(doc)
	}

	if err := indexer.Save(); err != nil {
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

	httpServer := srv.BuildHTTPServer(addr)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(ctx)
	}()

	slog.Info("starting server", "addr", addr)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
