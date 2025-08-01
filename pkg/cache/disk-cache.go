package cache

import (
	"container/list"
	"os"
	"sync"
)

type DiskCacheEntry struct {
	offset uint64
	elem   *list.Element
}

type DiskCache struct {
	file        *os.File
	data        []byte
	capacity    uint64
	mu          sync.Mutex
	index       map[string]*DiskCacheEntry
	lru         *list.List
	writeOffset uint64
}
