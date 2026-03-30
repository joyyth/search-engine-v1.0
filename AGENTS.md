# Agent Guidelines for search-engine-v1.0

This document provides guidelines for AI agents working on this codebase.

## Project Overview

This is a Go-based search engine that crawls web pages, builds an inverted index, and serves search queries via HTTP API. The project uses Go 1.26.1.

## Build, Lint, and Test Commands

### Running Tasks (via Taskfile.yml)

Most commands are run via `task` (Go task runner). Install from https://taskfile.dev/:

```bash
# Default: run verify pipeline (format check, vet, tests)
task

# Full CI pipeline with race detector and coverage
task ci

# Build the binary
task build

# Run the application
task run           # Run with CLI args: task run -- -serve
task run:crawl     # Crawl pkg.go.dev/std (-crawl)
task run:index     # Build index from crawled pages (-index)
task run:serve     # Start HTTP API (-serve) on :8082
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests verbosely
go test -v ./...

# Run a single test
go test -v ./internal/indexer -run TestLookup

# Run tests with race detector
go test -race ./...

# Run tests with coverage summary per package
go test -cover ./...

# Run benchmarks (if any)
go test -bench=. -benchmem ./...
```

### Linting and Formatting

```bash
# Format all Go sources (gofmt)
go fmt ./...

# Check formatting (fails if any file needs formatting)
task fmt:check

# Run go vet on all packages
go vet ./...

# Run golangci-lint (install from https://golangci-lint.run/ if missing)
golangci-lint run ./...

# Full check (format + vet + test)
task check
```

### Dependencies

```bash
# Tidy go.mod
go mod tidy

# Download modules
go mod download

# Verify module checksums
go mod verify
```

### Code Generation

```bash
# Run go generate (if you add //go:generate directives)
go generate ./...
```

### Cleanup

```bash
# Remove bin/ and coverage.out
task clean
```

## Code Style Guidelines

### Imports

- Group imports: standard library first, then third-party packages
- Use blank line between groups
- Example:

```go
import (
    "context"
    "flag"
    "log"
    "net/http"

    "github.com/joyyth/search-engine-v1.0/internal/api"
    "github.com/joyyth/search-engine-v1.0/internal/model"
)
```

### Formatting

- Use `gofmt` for automatic formatting
- Run `go fmt ./...` before committing
- Use `task fmt:check` to verify formatting in CI

### Naming Conventions

- **Packages**: lowercase, short, no underscores (e.g., `indexer`, `search`)
- **Types**: PascalCase (e.g., `Indexer`, `Search`, `Document`)
- **Functions/Methods**: PascalCase (e.g., `NewIndexer`, `Lookup`)
- **Variables/Constants**: camelCase or PascalCase for exported, camelCase for unexported
- **Interfaces**: typically ending with "-er" (e.g., `Store`, `Reader`)
- **Acronyms**: use all caps for 2-letter acronyms (e.g., `URL`, `HTTP`, `API`)
- **Tests**: `Test` prefix for test functions, `TestXxx` for table-driven subtests

### Types and Structures

- Use structs for data containers
- Use interfaces to define contracts (e.g., `Store` interface in `internal/store`)
- Prefer explicit types over `interface{}`
- Use pointers for large structs or when mutation is needed
- Document public types with comments

Example from codebase:

```go
type Indexer struct {
    index     map[string][]int
    documents []model.Document
    store     store.Store
}

type Document struct {
    ID      int
    URL     string
    Title   string
    Content string
}
```

### Error Handling

- Return errors explicitly, don't use `panic` in normal flow
- Use `fmt.Errorf` with `%w` for wrapped errors
- Handle errors at the appropriate level
- For fatal errors in main, use `log.Fatalf`
- For recoverable errors in libraries, return errors

Example:

```go
func (i *Indexer) Save() error {
    if err := i.store.SaveDocuments(i.documents); err != nil {
        return fmt.Errorf("saving documents: %w", err)
    }
    if err := i.store.SaveIndex(i.index); err != nil {
        return fmt.Errorf("saving index: %w", err)
    }
    return nil
}
```

### Testing

- Use table-driven tests when testing multiple cases
- Use `t.Run` for subtests
- Test naming: `TestXxx` where Xxx describes what's being tested
- Use descriptive test case names in tables
- Mock external dependencies (see `mockStore` in indexer_test.go)

Example:

```go
func TestLookup(t *testing.T) {
    idx := NewIndexer(&mockStore{})
    // ...

    tests := []struct {
        name    string
        token   string
        wantIDs []int
        wantOk  bool
    }{
        {name: "term in one document", token: "good", wantIDs: []int{0}, wantOk: true},
        // ...
    }

    for _, test := range tests {
        t.Run(test.name, func(t *testing.T) {
            ids, found := idx.Lookup(test.token)
            // assertions...
        })
    }
}
```

### Concurrency

- Document thread-safety guarantees
- The current `Indexer` is NOT concurrency-safe - callers must serialize access
- Use channels for goroutine communication
- Always handle shutdown properly (see graceful shutdown in main.go)

### Logging

- Use `log` package for logging
- Use appropriate log levels: `log.Printf` for info, `log.Fatalf` for fatal errors
- Include context in log messages

### Constants

- Group related constants with `const` blocks
- Use PascalCase for exported constants, camelCase for unexported

Example:

```go
const (
    docsPath  = "data/pages.json"
    indexPath = "data/index.json"
    startURL  = "https://pkg.go.dev/std"
)

const (
    allowedHost = "pkg.go.dev"
    maxPages    = 200
)
```

### Comments

- Comment public functions and types
- Use sentence-style comments (start with identifier name)
- Don't use block comments for simple explanations

### HTTP Servers

- Use standard library `net/http`
- Implement graceful shutdown (see `api/server.go`)
- Return appropriate HTTP status codes

### Project Structure

```
search-engine-v1.0/
├── cmd/
│   └── main.go           # Entry point
├── internal/
│   ├── api/              # HTTP server
│   ├── crawler/          # Web crawler
│   ├── indexer/          # Inverted index
│   ├── model/            # Data models
│   ├── search/           # Search logic
│   ├── store/            # Persistence
│   └── utils/            # Utilities
├── pkg/                  # Public packages (if any)
├── go.mod
├── go.sum
├── Taskfile.yml          # Task runner
└── AGENTS.md             # This file
```

## Important Notes

- This is a single-user search engine - the `Indexer` is NOT concurrency-safe
- The crawler is limited to `pkg.go.dev` domain
- Default crawl depth is 3, max pages is 200
- HTTP API runs on port 8082 by default
- Data is persisted to JSON files in `data/` directory
