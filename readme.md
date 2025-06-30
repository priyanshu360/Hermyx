# 🌀 Hermyx

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.20+-blue)](https://golang.org/)
[![Build](https://img.shields.io/badge/build-passing-brightgreen)]()
[![Status](https://img.shields.io/badge/status-beta-orange)]()

**Hermyx** is a blazing-fast, minimal reverse proxy with intelligent caching. Built using [`fasthttp`](https://github.com/valyala/fasthttp), it offers per-route configurability, graceful shutdown, and a clean YAML configuration system — perfect for modern microservices, edge routing, or lightweight API gateways.

---

## 🚀 Features

* ⚡ Ultra-fast request handling with [`fasthttp`](https://github.com/valyala/fasthttp)
* 🎯 Route-level proxy and cache control
* 🧠 Caching options: in-memory, disk, or Redis-based
* ⏱ TTL and capacity control per cache backend
* 🔍 Custom cache keys via `path`, `method`, `query`, and now even `header`
* 🪵 Flexible logging to file/stdout
* ✨ YAML config for simple deployments
* 🧹 Graceful shutdown with PID cleanup
* 🛠️ `init` command to scaffold config files

---

## 🧪 Examples

```bash
hermyx up --config ./configs/prod.yaml
hermyx down
hermyx init
```

---

## 📄 Configuration Guide

Hermyx is configured entirely through a YAML file.

### Example

```yaml
log:
  toFile: true
  filePath: "./hermyx.log"
  toStdout: true
  prefix: "[Hermyx]"
  flags: 0
  debugEnabled: true

server:
  port: 8080

storage:
  path: "./.hermyx"

cache:
  enabled: true
  type: "redis"
  ttl: 5m
  capacity: 1000
  maxContentSize: 1048576
  redis:
    address: "redis:6379"
    password: ""
    db: 0
    defaultTtl: 10s
    namespace: "hermyx:"
  keyConfig:
    type: ["path", "method", "query", "header"]
    headers:
      - key: "X-Request-User"
      - key: "X-Device-ID"
    excludeMethods: ["post", "put"]

routes:
  - name: "user-api"
    path: "^/api/users"
    target: "localhost:3000"
    include: [".*"]
    exclude: ["^/api/users/private"]
    cache:
      enabled: true
      ttl: 2m
      keyConfig:
        type: ["path", "query", "header"]
        headers:
          - key: "Authorization"
        excludeMethods: ["post"]
```

---

## 🧾 Configuration Reference

### `cache.keyConfig`

| Field            | Type                | Description                                                  |
| ---------------- | ------------------- | ------------------------------------------------------------ |
| `type`           | `[]string`          | Parts to form cache key: `path`, `method`, `query`, `header` |
| `excludeMethods` | `[]string`          | HTTP methods to skip caching                                 |
| `headers`        | `[]HeaderConfig`    | List of headers to include if `header` is in `type`          |

#### `HeaderConfig`

| Field | Type     | Description                        |
| ----- | -------- | ---------------------------------- |
| `key` | `string` | Header name to include in the key  |

---

## 🔁 How It Works

1. **Match**: Request path matched via route regex

2. **Filter**: Include/exclude filters applied

3. **Cache**:

   * Skip cache based on method or config
   * Key generated from selected parts:

     * `path` → request path
     * `method` → HTTP verb
     * `query` → query parameters
     * `header` → specific headers (e.g. `X-User-ID`, `Authorization`)
   * Cache lookup (Redis, memory, or disk)

4. **Respond**:

   * Serve from cache if hit
   * Otherwise proxy request to target
   * Store response in cache if eligible

5. **Header**: Response includes `X-Hermyx-Cache: HIT` or `MISS`

---

## 🧪 Debugging

* Enable `toStdout` and use `flags: 0` for human-readable logs
* Use `X-Hermyx-Cache` response header to check cache behavior
* Add custom headers like `X-User-ID` or `Authorization` for user-specific cache keys
* Redis TTL expiry observable via `redis-cli TTL <key>`

