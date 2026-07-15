package cachethem

import (
	"sync"
	"time"
)

// Cache is a high-performance, sharded, thread-safe LRU cache.
type Cache struct {
	shards    []*shard
	config    *config
	closeCh   chan struct{}
	closeOnce sync.Once
}

// New initializes a new Cache with the given options.
func New(opts ...Option) *Cache {
	// 1. Initialize config with default values
	cfg := &config{
		NumShards:        defaultNumShards,
		DefaultTTL:       defaultTTL,
		JanitorInterval:  defaultJanitorInterval,
		MaxShardCapacity: defaultMaxShardCapacity,
	}

	// 2. Apply user-provided functional options
	for _, opt := range opts {
		opt(cfg)
	}

	// 3. Validate config
	if cfg.NumShards == 0 {
		cfg.NumShards = 1
	}

	// 4. Initialize the shards
	c := &Cache{
		shards:  make([]*shard, cfg.NumShards),
		config:  cfg,
		closeCh: make(chan struct{}),
	}

	for i := 0; i < int(cfg.NumShards); i++ {
		c.shards[i] = newShard(cfg.MaxShardCapacity)
	}

	// 5. Start the background Janitor if the interval is > 0 and expiration is enabled
	if cfg.JanitorInterval > 0 && cfg.DefaultTTL > 0 {
		go c.janitor()
	}

	return c
}

// getShard uses the FNV-1a hash of the key to determine which shard should handle it.
func (c *Cache) getShard(key string) *shard {
	hash := fnv64a(key)
	return c.shards[hash%uint64(c.config.NumShards)]
}

// Set adds or updates an item in the cache. 
// If ttl is 0, the cache's default TTL is used. If ttl is < 0, the item never expires.
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	if ttl == 0 {
		ttl = c.config.DefaultTTL
	} else if ttl < 0 {
		ttl = 0 // Negative TTL means no expiration
	}
	
	shard := c.getShard(key)
	shard.set(key, value, ttl)
}

// Get retrieves an item from the cache.
// It returns the value and a boolean indicating if the key was found.
func (c *Cache) Get(key string) (any, bool) {
	shard := c.getShard(key)
	return shard.get(key)
}

// Delete explicitly removes an item from the cache.
func (c *Cache) Delete(key string) {
	shard := c.getShard(key)
	shard.delete(key)
}

// Close gracefully stops the background janitor goroutine.
// It is safe to call Close multiple times.
func (c *Cache) Close() {
	c.closeOnce.Do(func() { close(c.closeCh) })
}


func (c *Cache) janitor() {
	ticker := time.NewTicker(c.config.JanitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now().UnixNano()

			for _, s := range c.shards {
				s.deleteExpired(now)
			}
		case <-c.closeCh:
			return
		}
	}
}