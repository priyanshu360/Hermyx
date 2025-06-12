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
* 🔍 Custom cache keys via `path`, `method`, `query`
* 🪵 Flexible logging to file/stdout
* ✨ YAML config for simple deployments
* 🧹 Graceful shutdown with PID cleanup

---

## 📦 Installation

Currently, Hermyx can be built from source:

```bash
git clone https://github.com/spyder01/hermyx
cd hermyx
go build -o hermyx ./cmd/hermyx
````

---

## ⚙️ CLI Usage

```bash
hermyx <command> [--config <path>]
```

### Available Commands

| Command | Description                                     |
| ------- | ----------------------------------------------- |
| `up`    | Start the Hermyx reverse proxy                  |
| `down`  | Shut down the running Hermyx server gracefully  |
| `help`  | Show help for a command                         |

### Command Details

#### `up`

Start the Hermyx reverse proxy with the specified configuration file.

```bash
hermyx up --config path/to/hermyx.config.yaml
```

#### `down`

Gracefully shut down the running Hermyx server.

```bash
hermyx down --config path/to/hermyx.config.yaml
```

#### `help`

Show general help or command-specific help.

```bash
hermyx help
hermyx help up
hermyx help down
```

---

## 🧪 Examples

Start Hermyx with a custom config:

```bash
hermyx up --config ./configs/prod.yaml
```

Stop Hermyx with the default config path:

```bash
hermyx down
```

Get help for the `up` command:

```bash
hermyx help up
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

server:
  port: 8080

storage:
  path: "./.hermyx"

cache:
  enabled: true
  type: "redis" # "memory", "disk", or "redis"
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
    type: ["path", "method", "query"]
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
        type: ["path", "query"]
        excludeMethods: ["post"]
```

---

## 🧾 Configuration Reference

### `log`

| Field       | Type      | Description                   |
| ----------- | --------- | ----------------------------- |
| `toFile`    | `bool`    | Write logs to a file          |
| `filePath`  | `string`  | Log file path                 |
| `toStdout`  | `bool`    | Also log to stdout            |
| `prefix`    | `string`  | Log line prefix               |
| `flags`     | `int`     | Logging flags (Go log style)  |

---

### `server`

| Field  | Type  | Description        |
| ------ | ----- | ------------------ |
| `port` | `int` | Port to listen on  |

---

### `storage`

| Field  | Type      | Description                    |
| ------ | --------- | ------------------------------ |
| `path` | `string`  | Path for PID and temp storage  |

---

### `cache`

| Field             | Type      | Description                                                                                                                    |
| ----------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `type`            | `string`  | Cache backend: `"memory"`, `"disk"`, or `"redis"` (**global-only**)                                                            |
| `enabled`         | `bool`    | Enable global cache                                                                                                            |
| `ttl`             | `string`  | Default cache TTL (`1m`, `5s`, etc.)                                                                                           |
| `capacity`        | `int`     | Max entries (used in memory; optional in Redis)                                                                                |
| `maxContentSize`  | `int`     | Max response body size to cache (ignored in Redis)                                                                             |
| `redis`           | `object`  | Redis-specific configuration (only required if `type: redis`)                                                                  |
| `keyConfig`       | `object`  | See below                                                                                                                      |

#### `redis`

| Field         | Type     | Description                              |
| ------------- | -------- | ---------------------------------------- |
| `address`     | `string` | Redis server address (`host:port`)       |
| `password`    | `string` | Redis password (optional)                |
| `db`          | `int`    | Redis DB index (e.g. `0`)                |
| `defaultTtl`  | `string` | Default TTL for Redis entries            |
| `namespace`   | `string` | Prefix for Redis keys (for isolation)    |

#### `keyConfig`

| Field             | Type        | Description                                  |
| ----------------- | ----------- | -------------------------------------------- |
| `type`            | `[]string`  | Parts to form cache key (`path`, `query`)    |
| `excludeMethods`  | `[]string`  | HTTP methods to skip caching (`post`, etc.)  |

---

### `routes`

| Field      | Type        | Description                                             |
| ---------- | ----------- | ------------------------------------------------------- |
| `name`     | `string`    | Name for logging/debugging                              |
| `path`     | `string`    | Regex to match request path                             |
| `target`   | `string`    | Upstream server (host\:port)                            |
| `include`  | `[]string`  | Optional: only forward matching paths                   |
| `exclude`  | `[]string`  | Optional: exclude forwarding certain paths              |
| `cache`    | `object`    | Route-specific cache override (TTL and key config only) |

---

## 🔁 How It Works

1. **Match**: Request path matched via route regex
2. **Filter**: Include/exclude filters applied
3. **Cache**:

   * Skip cache based on method or config
   * Key generated from selected parts (path, method, query)
   * Cache lookup (Redis, memory, or disk)
4. **Respond**:

   * Serve from cache if hit
   * Otherwise proxy request to target
   * Store response in cache if eligible
5. **Header**: Response includes `X-Hermyx-Cache: HIT` or `MISS`

---

## 🧹 Graceful Shutdown

Hermyx handles shutdown cleanly:

* Captures `SIGINT` / `SIGTERM`
* Deletes PID file
* Logs shutdown
* Flushes logs

---

## 🧪 Debugging

* Enable `toStdout` and use `flags: 0` for human-readable logs
* Use `X-Hermyx-Cache` header to inspect cache behavior
* Use route-specific TTL for aggressive or lenient caching
* Redis TTL expiry observable via `redis-cli TTL <key>`

---

## 🧭 Roadmap

* [ ] TLS support (HTTPS)
* [ ] Prometheus metrics
* [ ] Disk-based persistent cache backend
* [ ] Redis clustering + failover support
* [ ] Built-in dashboard or admin API
* [ ] Route hot-reloading

---

## 📜 License

MIT © [Suhan Bangera](https://github.com/spyder01)
