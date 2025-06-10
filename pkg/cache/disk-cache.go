package cache

import (
	"container/list"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
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

func NewDiskCache(storagePath string, capacity uint64) (*DiskCache, error) {
	file, err := os.OpenFile(filepath.Join(storagePath, "hermyx.cache"), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if fileInfo.Size() == 0 {
		if err := file.Truncate(1 << 20); err != nil {
			file.Close()
			return nil, err
		}
	}

	data, err := syscall.Mmap(int(file.Fd()), 0, int(fileInfo.Size()), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, err
	}

	cache := &DiskCache{
		file:        file,
		data:        data,
		capacity:    capacity,
		index:       make(map[string]*DiskCacheEntry),
		lru:         list.New(),
		writeOffset: 0,
	}

	err = cache.loadIndices()
	if err != nil {
		return nil, err
	}

	return cache, nil
}

func (cache *DiskCache) loadIndices() error {
	offset := uint64(0)
	data := cache.data
	dataLen := uint64(len(data))

	for offset < dataLen && offset+4 < dataLen {
		keyLen := uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if keyLen <= 0 || offset+uint64(keyLen) > uint64(len(data)) {
			break
		}

		key := string(data[offset : offset+keyLen])
		offset += keyLen

		if offset+8 > dataLen {
			break
		}
		expiry := uint64(binary.BigEndian.Uint64(data[offset : offset+8]))
		offset += 8

		if offset+4 > dataLen {
			break
		}
		valLen := uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4

		if offset+uint64(valLen) > dataLen {
			break
		}

		if expiry != 0 && uint64(time.Now().UnixNano()) > expiry {
			offset += uint64(valLen)
			continue
		}

		entryOffset := offset - valLen - 4 - 8 - uint64(keyLen) - 4
		elem := cache.lru.PushFront(key)
		cache.index[key] = &DiskCacheEntry{offset: entryOffset, elem: elem}
		offset += valLen
		cache.writeOffset = offset

	}

	return nil
}

func (cache *DiskCache) Get(key string) ([]byte, bool, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	entry, found := cache.index[key]
	if !found {
		return nil, false, errors.New("key not found")
	}

	offset := entry.offset
	data := cache.data
	dataLen := uint64(len(data))

	if offset+4 > dataLen {
		return nil, false, errors.New("corrupted data")
	}
	keyLen := uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if offset+keyLen > dataLen {
		return nil, false, errors.New("corrupted data")
	}
	storedKey := string(data[offset : offset+keyLen])
	offset += uint64(keyLen)

	if storedKey != key {
		return nil, false, errors.New("key mismatch")
	}

	expiry := uint64(binary.BigEndian.Uint64(data[offset : offset+8]))
	offset += 8
	if expiry != 0 && uint64(time.Now().UnixNano()) > expiry {
		cache.delete(key)
		return nil, false, errors.New("key expired")
	}

	valLen := uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if offset+valLen > dataLen {
		return nil, false, errors.New("corrupted data")
	}

	val := make([]byte, valLen)
	copy(val, data[offset:offset+valLen])

	cache.lru.MoveToFront(entry.elem)
	return val, true, nil
}

func (cache *DiskCache) Set(key string, value []byte, ttl time.Duration) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if existing, found := cache.index[key]; found {
		cache.delete(key)
		_ = existing
	}

	keyLen := uint32(len(key))
	valLen := uint32(len(value))
	expiry := uint64(0)
	if ttl > 0 {
		expiry = uint64(time.Now().Add(ttl).UnixNano())
	}

	entrySize := 4 + len(key) + 8 + 4 + len(value)
	requiredSize := cache.writeOffset + uint64(entrySize)

	if requiredSize > uint64(len(cache.data)) {
		if err := cache.expandFile(requiredSize); err != nil {
			return err
		}
	}

	offset := cache.writeOffset
	data := cache.data

	binary.BigEndian.PutUint32(data[offset:offset+4], keyLen)
	offset += 4
	copy(data[offset:offset+uint64(keyLen)], key)
	offset += uint64(keyLen)

	binary.BigEndian.PutUint64(data[offset:offset+8], expiry)
	offset += 8

	binary.BigEndian.PutUint32(data[offset:offset+4], valLen)
	offset += 4
	copy(data[offset:offset+uint64(valLen)], value)
	offset += uint64(valLen)

	elem := cache.lru.PushFront(key)
	cache.index[key] = &DiskCacheEntry{offset: cache.writeOffset, elem: elem}

	cache.writeOffset = offset

	if uint64(cache.lru.Len()) > cache.capacity {
		tail := cache.lru.Back()
		if tail != nil {
			cache.delete(tail.Value.(string))
		}
	}

	return nil
}

func (cache *DiskCache) delete(key string) {
	entry, found := cache.index[key]
	if !found {
		return
	}
	cache.lru.Remove(entry.elem)
	delete(cache.index, key)
}

func (cache *DiskCache) Delete(key string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.delete(key)
}

func (cache *DiskCache) expandFile(newSize uint64) error {
	if cache.file == nil {
		return errors.New("file closed")
	}

	if err := syscall.Munmap(cache.data); err != nil {
		return err
	}

	if err := cache.file.Truncate(int64(newSize)); err != nil {
		return err
	}

	data, err := syscall.Mmap(int(cache.file.Fd()), 0, int(newSize), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return err
	}

	cache.data = data
	return nil
}

func (cache *DiskCache) Close() error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.data != nil {
		syscall.Munmap(cache.data)
		cache.data = nil
	}

	if cache.file != nil {
		err := cache.file.Sync()
		cache.file.Close()
		cache.file = nil
		return err
	}

	return nil
}
