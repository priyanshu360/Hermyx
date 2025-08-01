//go:build windows

package cache

import (
	"container/list"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func NewDiskCache(storagePath string, capacity uint64) (*DiskCache, error) {
	path := filepath.Join(storagePath, "hermyx.cache")
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	cache := &DiskCache{
		file:        file,
		capacity:    capacity,
		index:       make(map[string]*DiskCacheEntry),
		lru:         list.New(),
		writeOffset: uint64(stat.Size()),
	}

	err = cache.loadIndices()
	if err != nil {
		file.Close()
		return nil, err
	}

	return cache, nil
}

func (cache *DiskCache) loadIndices() error {
	offset := uint64(0)
	buf := make([]byte, 8)

	stat, err := cache.file.Stat()
	if err != nil {
		return err
	}
	fileSize := uint64(stat.Size())

	for {
		if offset+4 > fileSize {
			break
		}
		if _, err := cache.file.ReadAt(buf[:4], int64(offset)); err != nil {
			break
		}
		keyLen := binary.BigEndian.Uint32(buf[:4])
		offset += 4

		if offset+uint64(keyLen) > fileSize {
			break
		}
		keyBuf := make([]byte, keyLen)
		if _, err := cache.file.ReadAt(keyBuf, int64(offset)); err != nil {
			break
		}
		key := string(keyBuf)
		offset += uint64(keyLen)

		if offset+8 > fileSize {
			break
		}
		if _, err := cache.file.ReadAt(buf, int64(offset)); err != nil {
			break
		}
		expiry := binary.BigEndian.Uint64(buf)
		offset += 8

		if offset+4 > fileSize {
			break
		}
		if _, err := cache.file.ReadAt(buf[:4], int64(offset)); err != nil {
			break
		}
		valLen := binary.BigEndian.Uint32(buf[:4])
		offset += 4

		if offset+uint64(valLen) > fileSize {
			break
		}

		if expiry != 0 && uint64(time.Now().UnixNano()) > expiry {
			offset += uint64(valLen)
			continue
		}

		entryOffset := offset - 4 - 8 - uint64(keyLen) - 4
		elem := cache.lru.PushFront(key)
		cache.index[key] = &DiskCacheEntry{offset: entryOffset, elem: elem}
		offset += uint64(valLen)
		cache.writeOffset = offset
	}

	return nil
}

func (cache *DiskCache) Get(key string) ([]byte, bool, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	entry, found := cache.index[key]
	if !found {
		return nil, false, errors.New(fmt.Sprintf("key %s not found", key))
	}

	offset := entry.offset
	buf := make([]byte, 8)

	if _, err := cache.file.ReadAt(buf[:4], int64(offset)); err != nil {
		return nil, false, err
	}
	keyLen := binary.BigEndian.Uint32(buf[:4])
	offset += 4

	keyBuf := make([]byte, keyLen)
	if _, err := cache.file.ReadAt(keyBuf, int64(offset)); err != nil {
		return nil, false, err
	}
	storedKey := string(keyBuf)
	offset += uint64(keyLen)

	if storedKey != key {
		return nil, false, errors.New("key mismatch")
	}

	if _, err := cache.file.ReadAt(buf, int64(offset)); err != nil {
		return nil, false, err
	}
	expiry := binary.BigEndian.Uint64(buf)
	offset += 8

	if expiry != 0 && uint64(time.Now().UnixNano()) > expiry {
		cache.delete(key)
		return nil, false, errors.New("key expired")
	}

	if _, err := cache.file.ReadAt(buf[:4], int64(offset)); err != nil {
		return nil, false, err
	}
	valLen := binary.BigEndian.Uint32(buf[:4])
	offset += 4

	val := make([]byte, valLen)
	if _, err := cache.file.ReadAt(val, int64(offset)); err != nil {
		return nil, false, err
	}

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

	header := make([]byte, 4+len(key)+8+4)
	binary.BigEndian.PutUint32(header[0:], keyLen)
	copy(header[4:], key)
	binary.BigEndian.PutUint64(header[4+len(key):], expiry)
	binary.BigEndian.PutUint32(header[4+len(key)+8:], valLen)

	offset := cache.writeOffset
	if _, err := cache.file.WriteAt(header, int64(offset)); err != nil {
		return err
	}
	if _, err := cache.file.WriteAt(value, int64(offset+uint64(len(header)))); err != nil {
		return err
	}

	elem := cache.lru.PushFront(key)
	cache.index[key] = &DiskCacheEntry{offset: offset, elem: elem}
	cache.writeOffset += uint64(len(header)) + uint64(len(value))

	if uint64(cache.lru.Len()) > cache.capacity {
		if tail := cache.lru.Back(); tail != nil {
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

func (cache *DiskCache) Close() error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.file != nil {
		err := cache.file.Sync()
		cache.file.Close()
		cache.file = nil
		return err
	}
	return nil
}
