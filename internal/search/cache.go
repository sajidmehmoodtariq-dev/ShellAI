package search

import (
	"container/list"
	"sync"
)

type cacheItem struct {
	key   string
	value []ScoredMatch
}

type resultCache struct {
	capacity int
	mu       sync.Mutex
	lru      *list.List
	byKey    map[string]*list.Element
}

func newResultCache(capacity int) *resultCache {
	if capacity <= 0 {
		capacity = 1
	}
	return &resultCache{
		capacity: capacity,
		lru:      list.New(),
		byKey:    make(map[string]*list.Element, capacity),
	}
}

func (c *resultCache) Get(key string) ([]ScoredMatch, bool) {
	if c == nil || key == "" {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.byKey[key]
	if !ok {
		return nil, false
	}

	c.lru.MoveToFront(elem)
	item := elem.Value.(*cacheItem)
	return cloneMatches(item.value, len(item.value)), true
}

func (c *resultCache) Put(key string, value []ScoredMatch) {
	if c == nil || key == "" || len(value) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.byKey[key]; ok {
		item := elem.Value.(*cacheItem)
		item.value = cloneMatches(value, len(value))
		c.lru.MoveToFront(elem)
		return
	}

	elem := c.lru.PushFront(&cacheItem{key: key, value: cloneMatches(value, len(value))})
	c.byKey[key] = elem

	if c.lru.Len() <= c.capacity {
		return
	}

	last := c.lru.Back()
	if last == nil {
		return
	}
	item := last.Value.(*cacheItem)
	delete(c.byKey, item.key)
	c.lru.Remove(last)
}

func (c *resultCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lru.Len()
}
