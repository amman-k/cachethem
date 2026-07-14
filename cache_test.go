package cachethem

import (
	"strconv"
	"testing"
	"time"
)

// TestCache_GetSet uses table-driven testing to verify the core logic:
// creation, retrieval, updates, and expiration (TTL).
func TestCache_GetSet(t *testing.T) {
	// Initialize with a tiny default TTL for fast testing
	c := New(WithDefaultTTL(50*time.Millisecond), WithNumShards(4))
	defer c.Close()

	tests := []struct {
		name      string
		key       string
		val       interface{}
		ttl       time.Duration
		sleep     time.Duration // How long to wait before getting
		wantFound bool
		wantVal   interface{}
	}{
		{
			name:      "Set and get new item",
			key:       "user:1",
			val:       "alice",
			ttl:       0, // Use default
			sleep:     0,
			wantFound: true,
			wantVal:   "alice",
		},
		{
			name:      "Item expires due to TTL",
			key:       "user:2",
			val:       "bob",
			ttl:       10 * time.Millisecond,
			sleep:     20 * time.Millisecond,
			wantFound: false,
			wantVal:   nil,
		},
		{
			name:      "Update existing item",
			key:       "user:1", // Reusing key from test 1
			val:       "alice_updated",
			ttl:       0,
			sleep:     0,
			wantFound: true,
			wantVal:   "alice_updated",
		},
		{
			name:      "Negative TTL means no expiration",
			key:       "user:3",
			val:       "charlie",
			ttl:       -1,
			sleep:     60 * time.Millisecond, // Longer than default TTL
			wantFound: true,
			wantVal:   "charlie",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.Set(tt.key, tt.val, tt.ttl)

			if tt.sleep > 0 {
				time.Sleep(tt.sleep)
			}

			got, found := c.Get(tt.key)
			if found != tt.wantFound {
				t.Errorf("Get() found = %v, want %v", found, tt.wantFound)
			}
			if found && got != tt.wantVal {
				t.Errorf("Get() got = %v, want %v", got, tt.wantVal)
			}
		})
	}
}

// BenchmarkCacheGetSet tests performance under heavy concurrent load.
// Run with: go test -bench=. -benchmem
func BenchmarkCacheGetSet(b *testing.B) {
	// Configure for heavy load: 64 shards, 100k capacity per shard
	c := New(WithNumShards(64), WithMaxShardCapacity(100000))
	defer c.Close()

	b.ResetTimer() // Don't time the initialization

	// RunParallel executes the body in multiple goroutines simultaneously.
	b.RunParallel(func(pb *testing.PB) {
		var i int
		for pb.Next() {
			// Generate keys 0 through 9999 repeatedly
			key := strconv.Itoa(i % 10000)
			
			// Set the value
			c.Set(key, i, 0)
			
			// Immediately try to read it back
			c.Get(key)
			
			i++
		}
	})
}