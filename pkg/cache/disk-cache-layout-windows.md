# Hermyx Disk Cache File Format

This document explains the binary protocol used by the Hermyx disk cache system to persist cached entries to disk. The file format is designed for simplicity, linear appends, and fast lookup using an in-memory index.

---

## 📦 File Layout

The file consists of a sequence of cache entries written one after another. There is no header or footer in the file. Each entry has the following binary format:

```

\[Key Length (4 bytes)]
\[Key (variable length)]
\[Expiry Time (8 bytes)]
\[Value Length (4 bytes)]
\[Value (variable length)]

````

---

## 🧱 Entry Structure (in bytes)

| Field         | Size (bytes)         | Type          | Description                                                                 |
|---------------|----------------------|---------------|-----------------------------------------------------------------------------|
| Key Length    | 4                    | `uint32`      | Length of the key in bytes                                                  |
| Key           | variable (`N`)       | `[]byte`      | The UTF-8 encoded string key                                                |
| Expiry Time   | 8                    | `uint64`      | Expiry timestamp in **nanoseconds** since Unix epoch (0 = no expiry)       |
| Value Length  | 4                    | `uint32`      | Length of the value in bytes                                                |
| Value         | variable (`M`)       | `[]byte`      | Raw bytes of the value                                                      |

Total entry size = `4 + N + 8 + 4 + M` bytes.

---

## ⏱ Expiry Semantics

- An expiry time of `0` means the entry does **not** expire.
- Expiry is checked against the current system time using `time.Now().UnixNano()`.
- Expired entries are **skipped** during index loading and **evicted** during lookup.

---

## 🧭 Indexing

- At runtime, the cache reads the file **sequentially** on startup (`loadIndices()`), building an in-memory index (`map[string]*DiskCacheEntry`) and an LRU list.
- Each entry is tracked by its **offset** in the file, which points to the start of the entry (`KeyLen`).
- Expired entries are **not** loaded into the index.

---

## ♻️ Eviction (LRU)

- The cache maintains an in-memory Least Recently Used (LRU) list.
- If the number of entries exceeds the `capacity`, the **least recently used** key is evicted from both the index and LRU list.
- The evicted data is **not** removed from the file (it remains as unused space).

---

## 🔐 Concurrency

- All read and write operations are guarded using a `sync.Mutex` to ensure thread safety.
- The `Get`, `Set`, `Delete`, and `Close` methods acquire locks appropriately.

---

## ❌ Deletion

- Deleted or expired entries are not overwritten or removed from the file.
- The space they occupy is reclaimed only through file compaction or pruning (not implemented).

---

## 📁 File Naming

- The cache file is named `hermyx.cache` and created in the provided storage path.

---

## ✅ Platform

This version is built with `//go:build windows`, meaning it is explicitly for Windows OS. It does not use `mmap`, ensuring compatibility with the platform’s file I/O system.

---

## 💡 Example

Given a cache entry:

```go
key := "session123"
value := []byte("Hello, world!")
ttl := 5 * time.Minute
````

The file would contain:

| Component    | Example                                            |
| ------------ | -------------------------------------------------- |
| Key Length   | `0x0000000A` (10 bytes)                            |
| Key          | `"session123"`                                     |
| Expiry Time  | e.g., `0x0000018E29B1D5F0` (nanoseconds timestamp) |
| Value Length | `0x0000000D` (13 bytes)                            |
| Value        | `"Hello, world!"`                                  |

---

## 🛠 Future Enhancements

* **File compaction** to remove stale/dead entries
* **Checksums** for corruption detection
* **Memory-mapped I/O** for performance (Linux/macOS builds)
* **Concurrency-safe reads during writes** using RWMutex

---

## 📚 References

* Encoding: [`binary.BigEndian`](https://pkg.go.dev/encoding/binary)
* Timestamps: [`time.Now().UnixNano()`](https://pkg.go.dev/time#Time.UnixNano)

---
