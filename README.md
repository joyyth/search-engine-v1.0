# Search Engine

I tried building this to learn how search engines work. I was learning Go, so I aimed it at the Go docs (pkg.go.dev/std).

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

Needs Go 1.22+. From a clean clone:

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
    { "term": "articl", "doc_count": 187 },
    { "term": "page", "doc_count": 176 }
  ]
}
```

`doc_count` is document frequency (df), not term count — it is the length of the posting list for that term.

### `GET /health`

```bash
curl -s http://localhost:8082/health
```

```json
{ "status": "ok" }
```

### `GET /`

Serves the embedded UI (`internal/api/web/index.html`). Reads `?q=` from the URL so results are linkable.

---

## Notes

- BFS crawl with a worker pool; requests respect `context`.
- Same analyzer for index and query (tokenize, stopwords, Snowball English stem). TF in postings, IDF at query time; snippets use the raw query.
- JSON store with atomic replace; `Store` is an interface if you want something else later.
- Tests cover the indexer and the HTTP API (`httptest` against the real search path).

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

---

## Tech stack

| Area                   | Choice                                           | Reason                                                |
| ---------------------- | ------------------------------------------------ | ----------------------------------------------------- |
| Language               | Go 1.22+                                         | Learning Go                                           |
| Task runner (optional) | [go-task](https://taskfile.dev) (`Taskfile.yml`) | Not required — `go run ./cmd -serve` works standalone |

---

## License

[MIT](LICENSE) — Copyright (c) 2026 Tanvir Hassan Joy.
