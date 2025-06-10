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
* 🧠 In-memory caching with TTL, capacity, and size limits
* 🔍 Custom cache keys via `path`, `method`, `query`
* 🪵 Flexible logging to file/stdout
* ✨ YAML config for simple deployments
* 🧹 Graceful shutdown with PID cleanup

---

## 📦 Installation

Currently, Hermyx can be built from source:

```bash
git clone [https://github.com/spyder01/hermyx](https://github.com/spyder01/hermyx)
cd hermyx
go build -o hermyx ./cmd/hermyx
````

-----

## ⚙️ CLI Usage

```bash
hermyx <command> [--config <path>]
```

### Available Commands

| Command | Description                                    |
| ------- | ---------------------------------------------- |
| `up`    | Start the Hermyx reverse proxy                 |
| `down`  | Shut down the running Hermyx server gracefully |
| `help`  | Show help for a command                        |

### Command Details

#### `up`

Start the Hermyx reverse proxy with the specified configuration file.

```bash
hermyx up --config path/to/hermyx.config.yaml
```

  * `--config` (optional): Path to Hermyx config YAML file. Defaults to `./hermyx.config.yaml`.

#### `down`

Gracefully shut down the running Hermyx server.

```bash
hermyx down --config path/to/hermyx.config.yaml
```

  * `--config` (optional): Path to Hermyx config YAML file. Defaults to `./hermyx.config.yaml`.

#### `help`

Show general help or command-specific help.

```bash
hermyx help
hermyx help up
hermyx help down
```

-----

### Examples

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

-----

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
  type: "memory" # "memory" or "disk"
  ttl: 5m
  capacity: 1000 # This capacity is global
  maxContentSize: 1048576 # This maximum content size is global
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
      # Note: capacity and maxContentSize here would be ignored as they are global-only.
      keyConfig:
        type: ["path", "query"]
        excludeMethods: ["post"]
```

-----

## 🧾 Configuration Reference

### `log`

| Field      | Type     | Description                  |
| ---------- | -------- | ---------------------------- |
| `toFile`   | `bool`   | Write logs to a file         |
| `filePath` | `string` | Log file path                |
| `toStdout` | `bool`   | Also log to stdout           |
| `prefix`   | `string` | Log line prefix              |
| `flags`    | `int`    | Logging flags (Go log style) |

-----

### `server`

| Field  | Type  | Description       |
| ------ | ----- | ----------------- |
| `port` | `int` | Port to listen on |

-----

### `storage`

| Field  | Type     | Description                   |
| ------ | -------- | ----------------------------- |
| `path` | `string` | Path for PID and temp storage |

-----

### `cache`

| Field            | Type     | Description                                                                                                                    |
| ---------------- | -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `type`           | `string` | Cache storage type (`memory` or `disk`). **This setting is global only.** |
| `enabled`        | `bool`   | Enable global cache.                                                                                                                                       |
| `ttl`            | `string` | Default cache TTL (`1m`, `5s`, etc.).                                                                                                                      |
| `capacity`       | `int`    | Max entries in cache. **This setting is global only.** |
| `maxContentSize` | `int`    | Max size (in bytes) to cache. **This setting is global only.** |
| `keyConfig`      | `object` | See below.                                                                                                                                                 |

#### `keyConfig`

| Field            | Type       | Description                                 |
| ---------------- | ---------- | ------------------------------------------- |
| `type`           | `[]string` | Parts to form cache key (`path`, `query`)   |
| `excludeMethods` | `[]string` | HTTP methods to skip caching (`post`, etc.) |

-----

### `routes`

| Field     | Type       | Description                                |
| --------- | ---------- | ------------------------------------------ |
| `name`    | `string`   | Name for logging/debugging                 |
| `path`    | `string`   | Regex to match request path                |
| `target`  | `string`   | Upstream server (host:port)               |
| `include` | `[]string` | Optional: only forward matching paths      |
| `exclude` | `[]string` | Optional: exclude forwarding certain paths |
| `cache`   | `object`   | Route-specific override for cache settings. **Note: `type`, `capacity`, and `maxContentSize` defined here will be ignored as they are global-only.** |

-----

## 🔁 How It Works

1.  **Match**: Request path matched via route regex.
2.  **Filter**: Include/exclude filters applied.
3.  **Check Cache**: Cache eligibility based on method, size, etc.
4.  **Respond**:

   \* From cache if `HIT`
   \* Proxy to backend if `MISS`
5\. **Header**: Response includes `X-Hermyx-Cache: HIT` or `MISS`.

-----

## 🧹 Graceful Shutdown

Hermyx handles interrupts cleanly:

  * Captures `SIGINT` / `SIGTERM`
  * Deletes PID file
  * Logs shutdown
  * Flushes logs before exit

-----

## 🧪 Debugging

  * Enable `toStdout` and set `flags: 0` for readable logs.
  * Match errors or miss logs help diagnose cache misses.
  * Cache TTL expiry logs for fine-tuning.

-----

## 🧭 Roadmap

  * [ ] TLS support (HTTPS)
  * [ ] Prometheus metrics
  * [ ] Disk-based persistent cache backend
  * [ ] Built-in dashboard or admin API
  * [ ] Route hot-reloading

-----

## 📜 License

MIT © [Suhan Bangera](https://github.com/spyder01)

