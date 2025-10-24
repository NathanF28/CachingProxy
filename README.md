# CachingProxy

Simple HTTP caching proxy in Go.

This repository implements a minimal in-memory caching proxy that forwards requests to an origin server and caches responses in an LRU cache. It stores response bytes and returns cached responses with an `X-Cache` header indicating `MISS`, `HIT` or `BYPASS`.

## Features

- In-memory LRU cache (size 1000 by default).
- Default TTL for cached items: 5 minutes.
- Only caches `GET` requests.
- Honors `Cache-Control: no-store` / `no-cache` by bypassing cache.
- Prevents duplicate origin fetches with singleflight.
- Command-line flag to clear cache (note: this clears the cache on that proxy instance only).

## Prerequisites

- Go 1.25+ installed

## Build & Run

From the project root:

```powershell
# run the proxy forwarding to an example origin
go run main.go --port 8080 --origin http://httpbin.org
```

CLI flags (from `main.go`):
- `--port` (int) - port on which the proxy server will run
- `--origin` (string) - origin base URL to forward requests to
- `--clear-cache` (bool) - instantiates a proxy and calls ClearCache() then exits (does not affect a running server)

Example:

```powershell
go run main.go --port 8080 --origin http://dummyjson.com
```

## How to test the cache (manual)

1. Start the server:

```powershell
go run main.go --port 8080 --origin http://httpbin.org
```

2. Make a first request — expect `X-Cache: MISS`:

```powershell
curl -i http://localhost:8080/get
```

3. Repeat the same request — expect `X-Cache: HIT`:

```powershell
curl -i http://localhost:8080/get
```

4. Non-GET requests bypass the cache (forwarded live):

```powershell
curl -i -X POST http://localhost:8080/post -d '{"a":1}' -H "Content-Type: application/json"
```

5. Honor `Cache-Control` header (bypass):

```powershell
curl -i -H "Cache-Control: no-store" http://localhost:8080/get
```

Notes:
- The `--clear-cache` CLI flag runs ClearCache on a new proxy instance and exits. It does NOT clear the cache of a running server process. To clear a running server you can either restart it or add a runtime admin endpoint (e.g., `POST /-/cache/clear`) which is not present by default.

## Automated test (recommended)

You can add a test that uses `httptest` to simulate an origin and asserts that the first request is `MISS` and the second is `HIT`.

Example test file: `proxy/proxy_integration_test.go` (not included by default in repo):

```go
package proxy

import (
    "io"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestProxyCaching(t *testing.T) {
    origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/plain")
        w.WriteHeader(200)
        io.WriteString(w, "origin-body")
    }))
    defer origin.Close()

    p := NewProxy(origin.URL)

    // first request -> MISS
    req1 := httptest.NewRequest("GET", "/resource", nil)
    w1 := httptest.NewRecorder()
    p.ServeHTTP(w1, req1)
    resp1 := w1.Result()
    defer resp1.Body.Close()
    if got := resp1.Header.Get("X-Cache"); got != "MISS" {
        t.Fatalf("want X-Cache=MISS, got %q", got)
    }

    // second request -> HIT
    req2 := httptest.NewRequest("GET", "/resource", nil)
    w2 := httptest.NewRecorder()
    p.ServeHTTP(w2, req2)
    resp2 := w2.Result()
    defer resp2.Body.Close()
    if got := resp2.Header.Get("X-Cache"); got != "HIT" {
        t.Fatalf("want X-Cache=HIT, got %q", got)
    }
}
```

Run tests:

```powershell
go test ./... -run TestProxyCaching
```

## Implementation notes & caveats

- The cache stores a `*http.Response` and the response body bytes; storing the whole `*http.Response` can be brittle (some fields like `Body` are closed). The code stores the body bytes separately, which is safe for serving cached responses.
- TTL: cached items are considered expired after 5 minutes (`DefaultTTL`) and will be removed on the next lookup.
- LRU: the repository uses `github.com/hashicorp/golang-lru` with size 1000. Tune this value for memory requirements.
- Clearing cache at runtime: not implemented. Consider adding an admin HTTP endpoint to call `ProxyObject.ClearCache()`.
- Concurrency: singleflight is used to avoid duplicate origin calls when many concurrent cache misses occur for the same key.

## Next improvements (optional)

- Add an admin endpoint to clear the running cache (e.g., `POST /-/cache/clear`).
- Expose configuration for TTL and cache size via flags or environment variables.
- Improve stored response representation (status, headers, body) and avoid keeping `*http.Response` reference.
- Add metrics (cache hit ratio) and logging levels.

## License

Choose a license for your code (MIT, Apache-2.0, etc.).
