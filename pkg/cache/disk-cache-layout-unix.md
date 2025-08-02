# Hermyx Disk Cache File Format (Linux/macOS)

This document describes the binary file format and memory mapping protocol used by Hermyx's disk cache on **Linux** and **macOS** systems. It outlines how data is written, read, indexed, and managed using `mmap` for fast, low-overhead access.

---

## 📦 File Layout

The cache file (`hermyx.cache`) is a flat binary file structured as a **sequence of cache entries**, each containing the key, its expiry time, and the value.

Each cache entry has the following binary layout:

```

\[Key Length (4 bytes)]
\[Key (variable)]
\[Expiry Time (8 bytes)]
\[Value Length (4 bytes)]
\[Value (variable)]

````

---

## 🧱 Entry Structure (in bytes)

| Field         | Size (bytes)         | Type          | Description                                                                 |
|---------------|----------------------|---------------|-----------------------------------------------------------------------------|
| Key Length    | 4                    | `uint32`      | Length of the UTF-8 encoded key                                             |
| Key           | variable (`N`)       | `[]byte`      | The actual key                                                              |
| Expiry Time   | 8                    | `uint64`      | Expiration timestamp (in **nanoseconds** since epoch); `0` means no expiry |
| Value Length  | 4                    | `uint32`      | Length of the value in bytes                                                |
| Value         | variable (`M`)       | `[]byte`      | The cached data                                                             |

Total entry size = `4 + N + 8 + 4 + M` bytes.

---

## 🧠 Memory Mapping

- The cache file is **memory-mapped** using `syscall.Mmap` with `PROT_READ | PROT_WRITE` and `MAP_SHARED` flags.
- All reads and writes operate directly on the `[]byte` slice returned by `mmap`.
- The file is initialized to `1MB` (`1 << 20` bytes) on first run and **expanded** dynamically as needed.

---

## ⏱ Expiry Semantics

- The expiry timestamp is compared with `time.Now().UnixNano()`.
- If the value has expired:
  - It is **skipped** during index loading.
  - It is **evicted** during lookup (`Get`).
- A value of `0` means it **never expires**.

---

## 🧭 Indexing

- The entire file is scanned at startup (`loadIndices()`).
- An in-memory index (`map[string]*DiskCacheEntry`) and LRU list are built during this scan.
- Expired entries are **not added** to the index.
- The index maps each key to its **starting offset** in the mapped region.

---

## ♻️ Eviction (LRU)

- The cache maintains a Least Recently Used (LRU) list using `container/list`.
- If the number of entries exceeds the configured `capacity`, the **oldest** entry is evicted.
- Eviction only removes the entry from memory; the file data is **not deleted or compacted**.

---

## ❌ Deletion

- Deleted keys are only removed from the index and LRU list.
- The underlying file content remains untouched.
- Stale data may accumulate over time (future compaction could mitigate this).

---

## ⚠️ File Expansion

- When a new write would exceed the current mapped size:
  1. `syscall.Munmap` is called to unmap the file.
  2. The file is **truncated** to the new size.
  3. `syscall.Mmap` is called again with the new size.

- This allows seamless memory-backed growth of the file.

---

## 🧪 Example

Given a `Set("user123", []byte("hello"), 10 * time.Second)`, the written data would look like:

| Component     | Example Bytes                                           |
|---------------|---------------------------------------------------------|
| Key Length    | `0x00000007` (7 bytes)                                  |
| Key           | `"user123"`                                             |
| Expiry Time   | `0x0000018E2BCFE180` (timestamp in nanoseconds)         |
| Value Length  | `0x00000005` (5 bytes)                                  |
| Value         | `['h', 'e', 'l', 'l', 'o']`                             |

---

## 📁 File Naming

The cache file is named:

```text
hermyx.cache
````

It is stored in the user-provided `storagePath`.

---

## 🧹 Cleanup & Close

* `Close()`:

  * Calls `syscall.Munmap` to unmap the memory.
  * Syncs and closes the file.

---

## 💡 Future Enhancements

* **Compaction** to remove expired/deleted entries and reclaim disk space
* **Checksum** or CRC validation for corruption checks
* **Read-only mapping** for sharing across processes
* **Cross-platform compatibility wrappers**

---

## 🔐 Concurrency

* All public API methods (`Get`, `Set`, `Delete`, `Close`) are guarded by `sync.Mutex`.
* Ensures thread safety across goroutines.

---

## 📚 References

* [`syscall.Mmap`](https://pkg.go.dev/syscall#Mmap)
* [`encoding/binary`](https://pkg.go.dev/encoding/binary)
* [`time.Now().UnixNano()`](https://pkg.go.dev/time#Time.UnixNano)

---

## ✅ Platform

This implementation is **only for Linux and macOS**:

```go
//go:build linux || darwin
```

It uses memory-mapped I/O for performance and simplicity.

---
