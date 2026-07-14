package cachethem

import (
	"time"
)

const (
	defaultNumShards        = 32
	defaultTTL              = 5 * time.Minute
	defaultJanitorInterval  = 1 * time.Minute
	defaultMaxShardCapacity = 10000
)

// config holds the internal configuration for the cache.
type config struct {
	NumShards        uint32
	DefaultTTL       time.Duration
	JanitorInterval  time.Duration
	MaxShardCapacity int
}

// Option is a function that mutates the config. 
type Option func(*config)


func WithNumShards(shards uint32) Option {
	return func(c *config) {
		c.NumShards = shards
	}
}


func WithDefaultTTL(ttl time.Duration) Option {
	return func(c *config) {
		c.DefaultTTL = ttl
	}
}


func WithJanitorInterval(interval time.Duration) Option {
	return func(c *config) {
		c.JanitorInterval = interval
	}
}

func WithMaxShardCapacity(capacity int) Option {
	return func(c *config) {
		c.MaxShardCapacity = capacity
	}
}