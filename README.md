# Search Engine

A concurrent web crawler, inverted-index builder, and TF·IDF search API — written from scratch in Go, with a single-binary HTTP service and an embedded web UI.

Built to learn how production search actually works: a BFS crawler with a worker pool, a stemmed analyzer pipeline, postings with term frequency stored at index time, IDF computed at query time, and a ranked result feed served by `net/http` with middleware. Persistence is intentionally boring — JSON on disk behind a `Store` interface — so the interesting code (concurrency, ranking, lifecycle) stays legible. The web UI is shipped as a single HTML file compiled into the binary via `go:embed`, so `go run ./cmd -serve` produces a working search engine with zero external dependencies.

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

Requires Go 1.22+ (`go:embed` and method-scoped `ServeMux` patterns are used). Three commands from a clean clone:

```bash
go run ./cmd -crawl     # fetch up to 200 pages from en.wikipedia.org → data/pages.json
go run ./cmd -index     # build the inverted index              → data/index.json
go run ./cmd -serve     # start the API + UI on http://localhost:8082
```

Then open `http://localhost:8082` in a browser, or:

```bash
curl 'http://localhost:8082/search?q=concurrency'
```

All three flags can be combined (`go run ./cmd -crawl -index -serve`) to run the full pipeline in one process.

---

## API

### `GET /search?q={query}`

Returns the top 20 documents ranked by TF·IDF with a snippet centered on the highest-scoring raw-query token.

```bash
curl -s 'http://localhost:8082/search?q=goroutine' | jq .
```

```json
[
  {
    "Document": {
      "id": 42,
      "url": "https://en.wikipedia.org/wiki/Go_(programming_language)",
      "title": "Go (programming language) - Wikipedia",
      "content": "Go is a statically typed, compiled programming language..."
    },
    "Snippet": "...concurrency primitives such as goroutines and channels are first-class..."
  }
]
```

Errors:


| Status | Condition               |
| ------ | ----------------------- |
| 400    | `q` is missing or empty |
| 500    | JSON encoding failed    |


### `GET /stats`

Corpus-level metrics, useful for sanity-checking the index.

```bash
curl -s http://localhost:8082/stats | jq .
```

```json
{
  "total_documents": 200,
  "total_terms": 28734,
  "term_stats": [
    { "term": "wikipedia", "doc_count": 198 },
    { "term": "articl",    "doc_count": 187 },
    { "term": "page",      "doc_count": 176 }
  ]
}
```

`doc_count` is document frequency (df), not term count — it is the length of the posting list for that term.

### `GET /health`

```bash
curl -s http://localhost:8082/health
```

```json
{"status":"ok"}
```

### `GET /`

Serves the embedded single-file web UI (dark, minimal, vanilla JS — see `internal/api/web/index.html`). The UI reads `?q=` from the URL on load, so search results are deep-linkable.

---

## Docker

A multi-stage build produces a ~10 MB static image with no Go toolchain and no shell:

```dockerfile
# Dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/search-engine ./cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/search-engine /search-engine
EXPOSE 8082
ENTRYPOINT ["/search-engine"]
```

Build and run the full pipeline, persisting the crawl/index to a host volume:

```bash
docker build -t search-engine .

# crawl (~2 minutes on a fresh network)
docker run --rm -v "$PWD/data:/data" -w / search-engine -crawl

# index
docker run --rm -v "$PWD/data:/data" -w / search-engine -index

# serve on :8082
docker run --rm -p 8082:8082 -v "$PWD/data:/data" -w / search-engine -serve
```

The container writes to `/data` (configured by the `docsPath`/`indexPath` constants in `cmd/main.go`), so mounting a host directory there survives restarts.

---

## Technical highlights

- **Deadlock-free worker pool orchestrator.** The BFS loop in `internal/crawler/crawler.go` uses a single `select` over `results` receive and `jobs[0]` send with a `pending` counter as the barrier. When the queue is empty the loop blocks only on `results`; when both are zero it exits cleanly. Closing `jobs` after the main loop and waiting on a `sync.WaitGroup` guarantees every worker terminates before `close(results)`.
- **Context-cancellable crawl.** `Crawl(ctx, seed, workers)` plumbs `context.Context` into every `http.Request`, so shutdown and timeouts are first-class rather than bolted on.
- **Lock-free fetch statistics.** `crawlStats` is four `atomic.Int32` counters (`started`, `succeeded`, `requestErrors`, `nonOK`). Workers increment without contending on the page-slice mutex; a snapshot type is used for logging so readers never observe half-updated state.
- **TF at index, IDF at query.** `model.Posting{DocID, TF}` stores term frequency once per (doc, term). IDF is `log(1 + N/df)` computed on demand in `internal/search/search.go`, so adding documents does not require rewriting postings. The `1 +` smoothing avoids the log(1)=0 pathology where a term present in every document contributes nothing.
- **Analyzer consistency across index and query.** Index-time (`indexer.AddDocument`) and query-time (`search.Search`) both call the same `Analyzer(text)` — lowercase → `strings.FieldsFunc` on non-letter/digit → stopword filter → Snowball English stem. There is literally one code path, so there is no drift to debug.
- **Snippet uses the raw query, not stemmed tokens.** `BuildSnippet` receives `strings.Fields(strings.ToLower(query))` so the excerpt shows words the user actually typed, even though matching is stem-based.
- **Atomic, crash-safe persistence.** `JSONStore.writeJSON` encodes to `path + ".tmp"`, fsyncs, closes, then `os.Rename`s. A crash mid-write cannot corrupt the previous good index.
- `**Store` is an interface, not a package.** `internal/store/store.go` declares the contract; `JSONStore` is one implementation, a `mockStore` in `indexer_test.go` and a `testStore` in `server_test.go` are others. Swapping in BoltDB or SQLite is a new file, not a rewrite.
- **Zero-asset binary via `go:embed`.** `internal/api/server.go` embeds `web/index.html` at compile time. `go build ./cmd` produces one file you can `scp` anywhere.
- **CORS preflight handled correctly.** The `withCORS` middleware short-circuits `OPTIONS` with 204 before the Go 1.22 method-scoped mux can 405 it — browsers using custom headers actually work.
- **Structured logging everywhere.** `slog` JSON handler in `main`, per-request `slog.Info` in `withLogging`, per-fetch `slog.Info` in the crawler with duration, URL, depth, and status. `grep` becomes `jq`.
- **Tested where it matters.** `indexer` has analyzer and lookup tests; `api` has end-to-end tests driving `httptest.NewServer` against a real `Search` wired to an in-memory index — the same code path the running server uses.

---

## What I would build next

In priority order, with the reasoning:

1. **BM25 ranking behind a `?ranker=` query parameter.** TF·IDF ignores document length; a 100-word page with three hits beats a 10,000-word page with twenty. BM25 fixes this in ~40 lines and gives a measurable MRR improvement over a labeled sample. TF·IDF stays as the baseline for A/B comparison.
2. **Incremental indexing via `POST /index`.** Today `AddDocument` is not safe for concurrent use and only runs offline. Putting it behind a `sync.RWMutex` and exposing it over HTTP would turn this from a batch toy into something that looks like a real service.
3. **Prometheus `/metrics` endpoint.** Request count, p50/p99 latency, index size, crawl rate. Middleware wraps `ResponseWriter` to capture status codes. Opens the door to meaningful dashboards and is the single most signal-giving addition for an SRE-adjacent reviewer.
4. **BoltDB `Store` implementation.** Proves the `Store` interface is real and not decorative. Sub-millisecond term lookup on >100k-doc corpora without loading the entire index into memory.
5. **Autocomplete (`GET /suggest?q=`).** A prefix trie built at load time over unique index terms. Small, visible in the UI, and a nice opportunity to talk about trade-offs between tries and FSTs.
6. **Crawler politeness.** `robots.txt` parsing and per-host rate limiting via `golang.org/x/time/rate`. Required for a real crawler; right now the scope is capped by `MaxPages` rather than by etiquette.
7. **Query expansion with synonyms.** Small hand-curated synonym map applied after tokenization. Improves recall for short queries, demonstrates awareness of where pure lexical search breaks down.

Things I deliberately will not do: add a heavyweight framework, adopt a persistence layer just for the sake of it, or rewrite the crawler in a "cleaner" way. The current architecture is small enough to hold in your head — that is the feature.

---

## Tech stack


| Area                   | Choice                                                  | Reason                                                               |
| ---------------------- | ------------------------------------------------------- | -------------------------------------------------------------------- |
| Language               | Go 1.22+                                                | `go:embed`, method-scoped `ServeMux`, stdlib concurrency             |
| HTTP server            | `net/http` stdlib                                       | No framework — routing, middleware, and shutdown all fit in one file |
| HTML parsing           | `golang.org/x/net/html`                                 | Token-level walker; no CSS selectors needed for title + links + text |
| Stemming               | `github.com/kljensen/snowball` (English)                | Small, pure-Go, standard Porter2 algorithm                           |
| Logging                | `log/slog` (JSON handler)                               | Structured, stdlib, zero dependencies                                |
| Concurrency primitives | `sync.Mutex`, `sync.WaitGroup`, `sync/atomic`, channels | Matches what each code path actually needs                           |
| Persistence            | JSON files via a `Store` interface                      | Readable on disk, swappable at the interface boundary                |
| Web UI                 | Single HTML file, vanilla JS, `go:embed`                | Ships inside the binary, no build step                               |
| Task runner (optional) | [go-task](https://taskfile.dev) (`Taskfile.yml`)        | Not required — `go run ./cmd -serve` works standalone                |
| Container image        | `distroless/static-debian12:nonroot`                    | Static binary, non-root by default, no shell, ~10 MB                 |


---

## Project layout

```
cmd/main.go                       flag-based entrypoint (-crawl / -index / -serve)
internal/crawler/crawler.go       BFS worker pool, HTML walk, atomic stats
internal/model/document.go        Document{ID,URL,Title,Content}, Posting{DocID,TF}
internal/indexer/analyzer.go      tokenize → stopwords → Snowball stem
internal/indexer/indexer.go       inverted index + stats
internal/search/search.go         TF·IDF ranking + snippet assembly
internal/store/store.go           Store interface
internal/store/jsonstore.go       JSON impl with atomic tmp+rename writes
internal/api/server.go            HTTP API, CORS + logging middleware, go:embed UI
internal/api/web/index.html       single-file web UI
internal/utils/                   NormalizeURL, BuildSnippet
```

## License

[MIT](LICENSE) — Copyright (c) 2026 Tanvir Hassan Joy.