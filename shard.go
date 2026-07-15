package cachethem

import (
	"container/list"
	"sync"
	"time"
)

type item struct {
	key      string
	value    any
	expireAt int64
}

func (i *item) isExpired(now int64) bool {
	return i.expireAt > 0 && now > i.expireAt
}

type shard struct {
	mu       sync.RWMutex
	capacity int
	ll       *list.List
	cache    map[string]*list.Element
}

func newShard(capacity int) *shard {
	return &shard{
		capacity: capacity,
		ll:       list.New(),
		cache:    make(map[string]*list.Element),
	}
}

// set adds a new value or updates an existing one.
func (s *shard) set(key string, value any, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var expireAt int64
	if ttl > 0 {
		expireAt = time.Now().Add(ttl).UnixNano()
	}
	if ele, found := s.cache[key]; found {
		s.ll.MoveToFront(ele)
		it := ele.Value.(*item)
		it.value = value
		it.expireAt = expireAt
		return
	}
	it := &item{
		key:      key,
		value:    value,
		expireAt: expireAt,
	}
	ele := s.ll.PushFront(it)
	s.cache[key] = ele

	// LRU Eviction: If we exceeded capacity, pop the tail.
	if s.capacity > 0 && s.ll.Len() > s.capacity {
		s.removeOldest()
	}
}

func (s *shard) get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ele, found := s.cache[key]
	if !found {
		return nil, false
	}

	it := ele.Value.(*item)

	if it.isExpired(time.Now().UnixNano()) {
		s.removeElement(ele)
		return nil, false
	}

	// Mark as recently used
	s.ll.MoveToFront(ele)
	return it.value, true
}

func (s *shard) delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ele, found := s.cache[key]; found {
		s.removeElement(ele)
	}
}

func (s *shard) removeOldest() {
	ele := s.ll.Back()
	if ele != nil {
		s.removeElement(ele)
	}
}

func (s *shard) removeElement(ele *list.Element) {
	s.ll.Remove(ele)
	it := ele.Value.(*item)
	delete(s.cache, it.key)
}


func (s *shard) deleteExpired(now int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var next *list.Element
	for ele := s.ll.Back(); ele != nil; ele = next {
		next = ele.Prev()
		if ele.Value.(*item).isExpired(now) {
			s.removeElement(ele)
		}
	}
}