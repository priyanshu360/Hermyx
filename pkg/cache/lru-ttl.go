package cache

import (
	"container/list"
	"sync"
	"time"
)

type entry struct {
	key       string
	value     []byte
	expiresAt time.Time
	element   *list.Element
}

type Cache struct {
	capacity uint64
	mu       sync.Mutex
	items    map[string]*entry
	order    *list.List
}

func NewCache(capacity uint64) *Cache {
	if capacity <= 0 {
		panic("capacity must be > 0")
	}
	return &Cache{
		capacity: capacity,
		items:    make(map[string]*entry),
		order:    list.New(),
	}
}

func (c *Cache) Set(key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.items[key]; ok {
		e.value = value
		e.expiresAt = time.Now().Add(ttl)
		c.order.MoveToFront(e.element)
		return nil
	}

	if len(c.items) >= int(c.capacity) {
		c.evict()
	}

	elem := c.order.PushFront(key)
	c.items[key] = &entry{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(ttl),
		element:   elem,
	}

	return nil
}

func (c *Cache) Get(key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.items[key]
	if !ok || time.Now().After(e.expiresAt) {
		if ok {
			c.remove(key)
		}
		return nil, false, nil
	}

	c.order.MoveToFront(e.element)
	return e.value, true, nil
}

func (c *Cache) remove(key string) {
	e := c.items[key]
	c.order.Remove(e.element)
	delete(c.items, key)
}

func (c *Cache) evict() {
	back := c.order.Back()
	if back == nil {
		return
	}
	key := back.Value.(string)
	c.remove(key)
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[key]; ok {
		c.remove(key)
	}
}

func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}
