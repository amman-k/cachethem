# CacheThem

**A lightweight, in-memory caching library for Go with TTL support, pluggable eviction policies, and zero external dependencies.**

---

## Why CacheThem?

Most Go services eventually need a caching layer — for database query results, expensive computations, or API responses. Reaching for a full distributed cache (Redis, Memcached) adds operational overhead that isn't always justified, especially for single-instance services or hot-path data that lives comfortably in memory.

CacheThem fills that gap: a small, dependency-free, in-process cache that gives you TTL expiration, configurable eviction (LRU/LFU), and safe concurrent access, without pulling in a network dependency or a heavyweight framework. It's built for the common case — "I need a fast, safe map with expiration" — and gets out of your way otherwise.

If you outgrow it (multi-instance cache coherence, persistence, huge datasets), CacheThem is a good stepping stone before you reach for a distributed solution.

---

## Table of Contents

- [Why CacheThem?](#why-cachethem)
- [Core Features](#core-features)
- [Installation](#installation)
- [Supported Platforms](#supported-platforms)
- [Requirements](#requirements)
- [Quick Start](#quick-start)
- [Usage](#usage)
  - [Basic Example](#basic-example)
  - [Common Use Cases](#common-use-cases)
  - [Configuration Options](#configuration-options)
- [Architecture](#architecture)
- [Performance Considerations](#performance-considerations)
- [Security Considerations](#security-considerations)
- [FAQ](#faq)
- [Troubleshooting](#troubleshooting)
- [Testing](#testing)
- [Development Setup](#development-setup)
- [License](#license)
- [Acknowledgements](#acknowledgements)

---

## Core Features

- **TTL-based expiration** — set per-key or default time-to-live; expired entries are evicted lazily and via a background sweeper.
- **Pluggable eviction policies** — LRU and LFU built in; implement the `EvictionPolicy` interface for custom strategies.
- **Thread-safe** — safe for concurrent use across goroutines via sharded locking.
- **Zero external dependencies** — only the Go standard library.
- **Size-bounded caches** — cap by item count or approximate memory usage.
- **Generics support** — type-safe caches via Go generics (`Cache[K, V]`), no `interface{}` casting.
- **Metrics hooks** — optional callbacks for hits, misses, and evictions to feed your own monitoring.
- **Context-aware operations** — `Get`/`Set` variants that respect `context.Context` cancellation.

---

## Installation

Requires Go 1.21 or later (for generics support).

```bash
go get github.com/amman-k/cachethem@latest
```

Then import it:

```go
import "github.com/amman-k/cachethem"
```

That's it — no configuration files, no external services to run.

---

## Supported Platforms

CacheThem is pure Go and has no OS-specific code, so it runs anywhere the Go toolchain does:

- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)
- Any platform supported by Go 1.21+

---

## Requirements

- Go **1.21+** (uses generics and the `slices`/`maps` standard library packages)
- No external services, databases, or system dependencies

**Dependencies:** none beyond the Go standard library. Test-only dependencies (e.g., `testify`) are declared in `go.mod` under a separate build tag and are not required for production use.

---

## Quick Start

The simplest possible use case — a cache with a default TTL:

```go
package main

import (
	"fmt"
	"time"

	"github.com/amman-k/cachethem"
)

func main() {
	c := cachethem.New[string, string](cachethem.Config{
		DefaultTTL: 5 * time.Minute,
	})

	c.Set("user:42", "Jane Doe")

	if val, ok := c.Get("user:42"); ok {
		fmt.Println(val) // "Jane Doe"
	}
}
```

Run it with:

```bash
go run main.go
```

---

## Usage

### Basic Example

```go
c := cachethem.New[string, int](cachethem.Config{
	DefaultTTL: time.Minute,
	MaxItems:   1000,
})

c.Set("requests:count", 1)

val, found := c.Get("requests:count")
if !found {
	// key missing or expired
}

c.Delete("requests:count")
```

### Common Use Cases

- Caching results of expensive database queries within a single service instance
- Memoizing computation-heavy function calls (e.g., report generation, template rendering)
- Rate-limiting counters with TTL-based reset
- Short-lived session or token caches
- De-duplicating in-flight identical requests (combined with `GetOrCompute`)

### Configuration Options

| Option           | Type              | Default        | Description                                      |
|------------------|-------------------|----------------|--------------------------------------------------|
| `DefaultTTL`     | `time.Duration`   | `0` (no expiry)| TTL applied to keys set without an explicit TTL   |
| `MaxItems`       | `int`             | `0` (unbounded)| Maximum number of items before eviction triggers  |
| `EvictionPolicy` | `EvictionPolicy`  | `LRU`          | Strategy used when `MaxItems` is exceeded         |
| `CleanupInterval`| `time.Duration`   | `1 * time.Minute` | How often the background sweeper removes expired keys |
| `OnEvict`        | `func(K, EvictReason)` | `nil`     | Callback invoked whenever an item is evicted      |
| `ShardCount`     | `int`             | `32`           | Number of internal shards for lock contention control |


---

## Architecture

CacheThem shards its internal storage into multiple independent maps (default: 16), each protected by its own mutex. Keys are routed to shards via a hash function, which reduces lock contention under concurrent access compared to a single global lock.

```
            ┌─────────────┐
  Set/Get → │  Hash(key)  │
            └──────┬──────┘
                    │
         ┌──────────┼──────────┐
         ▼          ▼          ▼
     Shard 0     Shard 1  ...  Shard N
   (map+mutex) (map+mutex)  (map+mutex)
```

A background goroutine sweeps expired entries on `CleanupInterval`. Expired entries not yet swept are also treated as absent on `Get`, so correctness doesn't depend on sweep timing.

**Note:** `Close()` stops the background sweeper. Always call it (e.g., via `defer c.Close()`) when a cache instance goes out of scope, or you'll leak a goroutine.

---

## Performance Considerations

- CacheThem trades some memory overhead (sharding, per-entry metadata) for reduced lock contention. For low-concurrency workloads, a single-shard config (`ShardCount: 1`) may be marginally faster and use less memory.
- `MaxItems` eviction has O(1) amortized cost for LRU; LFU eviction is O(log n) due to the internal frequency heap.
- Very short `CleanupInterval` values increase background CPU usage on large caches. Start with the default (1 minute) and tune based on your expiration rate.
- CacheThem is an in-process cache — it does **not** reduce load across multiple service instances. If you run several replicas, each maintains its own independent cache.

---

## Security Considerations

- CacheThem does not encrypt cached values. If you cache sensitive data (tokens, PII), ensure the values themselves are already protected appropriately (encrypted at rest if persisted elsewhere, redacted in logs, etc.).
- There is no built-in access control — anything with a reference to the `Cache` instance can read or write any key. Scope cache instances accordingly within your application.
- Because eviction and expiration are size/time bound, an attacker who can influence cache keys (e.g., via user input) could attempt a cache-flooding pattern. Set a sensible `MaxItems` to bound worst-case memory usage.

---

## FAQ

**Does CacheThem support distributed caching across multiple nodes?**
No. It's an in-process cache only. For multi-node cache coherence, use Redis, Memcached, or a similar external store.

**Can I persist the cache to disk?**
Not currently. All data is lost on process restart. This is intentional — CacheThem targets ephemeral, hot-path caching.

**Does it work with non-comparable key types (e.g., slices, maps)?**
No. Keys must satisfy Go's `comparable` constraint, same as native Go maps.

**What happens if I don't set `MaxItems`?**
The cache grows unbounded except for TTL-based expiration. Set `MaxItems` in any long-running service to guard against unexpected growth.

**Is `Cache[K, V]` safe to share across goroutines?**
Yes, all exported methods are safe for concurrent use.

---

## Troubleshooting

| Symptom                                   | Likely Cause                                        | Fix                                                              |
|--------------------------------------------|------------------------------------------------------|-------------------------------------------------------------------|
| Memory grows unbounded                     | `MaxItems` not set, or `DefaultTTL` is `0`           | Set `MaxItems` and/or a `DefaultTTL`                              |
| Keys expire "too early" under load          | System clock drift or very short TTLs               | Verify TTL values; check for clock sync issues in containers      |
| Goroutine leak detected in profiling        | `Close()` was never called                           | Always `defer c.Close()` after creating a cache instance          |
| High lock contention under heavy concurrency | `ShardCount` too low for workload                    | Increase `ShardCount` (e.g., to number of CPU cores or higher)    |
| `GetOrCompute` runs the function twice for the same key | Expected under a race the first time two goroutines miss simultaneously | This is a known trade-off; wrap `fn` in your own `singleflight` group if strict single-execution is required |

---

## Testing

Run the full test suite:

```bash
go test ./...
```

Run with race detection (recommended before submitting a PR):

```bash
go test -race ./...
```

Run benchmarks:

```bash
go test -bench=. -benchmem ./...
```

---

## Development Setup

```bash
git clone https://github.com/amman-k/cachethem.git
cd cachethem
go mod download
go build ./...
```

Before submitting changes:

```bash
go vet ./...
gofmt -l .
go test -race ./...
```



## License

Released under the [MIT License](LICENSE).

---

## Acknowledgements

- Inspired by patterns from Go's `sync.Map`, `groupcache`, and `go-cache`.
- Hashing logic inspired by the Fowler–Noll–Vo hash function.
