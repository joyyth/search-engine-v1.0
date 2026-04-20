[![Go Reference](https://pkg.go.dev/badge/github.com/joyyth/search-engine-v1.0.svg)](https://pkg.go.dev/github.com/joyyth/search-engine-v1.0) [![Go Report Card](https://goreportcard.com/badge/github.com/joyyth/search-engine-v1.0)](https://goreportcard.com/report/github.com/joyyth/search-engine-v1.0)

# Search Engine

I tried building this to learn how search engines work. I am learning Go, so I aimed it at the Go docs (pkg.go.dev/std).

![Search UI showing pkg.go.dev results for hash](public/search-ui.png)

---

## Architecture

```
 seed URL
    │
    │  -crawl
    ▼
┌──────────────────┐      jobs chan       ┌──────────────────┐
│   Orchestrator   │ ───────────────────▶ │   Worker Pool    │
│   BFS + dedup    │                      │   HTTP fetch,    │
│   pending count  │ ◀─────────────────── │   HTML walk,     │
│   (select loop)  │     results chan     │   link extract   │
└────────┬─────────┘                      └──────────────────┘
         │
         ▼
    data/pages.json        ◀── JSONStore (atomic tmp + rename)
         │
         │  -index
         ▼
┌──────────────────────────────────────────────────┐
│  Indexer                                          │
│    Analyzer:  lowercase → tokenize                │
│               → stopword filter                   │
│               → Snowball stem                     │
│    Postings:  term → [{docID, tf}]                │
│    TF computed at index time, stored per posting  │
└────────┬─────────────────────────────────────────┘
         │
         ▼
    data/index.json
         │
         │  -serve
         ▼
┌──────────────────────────────────────────────────┐
│  HTTP API ( :8082 )                               │
│    middleware: CORS (preflight-safe), slog log    │
│                                                   │
│    GET /          → embedded web UI (go:embed)    │
│    GET /search?q= → TF·IDF ranking + snippets     │
│    GET /stats     → doc count, top terms          │
│    GET /health    → liveness                      │
│                                                   │
│  IDF = log(1 + N/df) computed per query           │
└──────────────────────────────────────────────────┘
```

Data flows **left to right, once per mode**: crawl writes `pages.json`, index reads it and writes `index.json`, serve loads both into memory and answers queries. Each stage is a flag on the same binary (`-crawl`, `-index`, `-serve`) so you can run them in sequence or separately.

---

## Quickstart

- Clone the repository by running `git clone https://github.com/joyyth/search-engine-v1.0`
- cd search-engine-v1.0

```bash
go run ./cmd -crawl     # fetch up to 200 pages from pkg.go.dev/std → data/pages.json
go run ./cmd -index     # build the inverted index              → data/index.json
go run ./cmd -serve     # start the API + UI on http://localhost:8082
```

Then open `http://localhost:8082` in a browser, or:

```bash
curl 'http://localhost:8082/search?q=hash'
```

All three flags can be combined (`go run ./cmd -crawl -index -serve`) to run the full pipeline in one process.

---

### Services and workflows

- **Crawler Workers**: Crawls pages from the seed URL with a breadth-first strategy and deduplicates visited URLs. It fetches HTML pages, extracts links, and persists crawled documents to `data/pages.json` via the JSON store.
- **Indexer**: Reads crawled pages from `data/pages.json` and builds an inverted index (`term -> postings`). It applies lowercase/tokenization/stopword filtering/stemming and writes the output to `data/index.json`.
- **TF-IDF Ranker**: Uses term frequency stored in postings and computes inverse document frequency at query time to score documents for a search query.
- **Query Engine (API)**: Serves `/search`, validates query input, runs retrieval and ranking, boosts ranking scores for pkg.go.dev-style URLs and titles, and builds snippets.The top ranked matches are returned with snippets.
- **Stats Service**: Exposes corpus/index metrics through `/stats` so you can inspect total documents, total terms, and high document-frequency terms.
- **Client Web UI**: Serves the search interface at `/` and displays ranked results from the API.

---

## Tech stack

- Go
- HTML/CSS/JS for the client-side web interface.
- Taskfile for running the project. (not required - go run ./cmd -serve works standalone)

---

## Plans

1. **BM25 ranking behind a `?ranker=` query parameter.**
2. **BoltDB/BadgerDB `Store` implementation.**
3. **Autocomplete (`GET /suggest?q=`).** A prefix trie built at load time over unique index terms.
