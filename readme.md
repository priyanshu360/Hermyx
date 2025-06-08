# Hermyx

[![Go Reference](https://pkg.go.dev/badge/github.com/yourorg/hermyx.svg)](https://pkg.go.dev/github.com/yourorg/hermyx)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Hermyx is a lightweight, high-performance HTTP reverse proxy and caching engine written in Go. It proxies requests to backend servers with configurable route matching, filtering, and in-memory response caching.

---

## Table of Contents

- [Features](#features)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Configuration](#configuration)
  - [Running Hermyx](#running-hermyx)
- [How It Works](#how-it-works)
- [Project Structure](#project-structure)
- [Extending Hermyx](#extending-hermyx)
- [Contributing](#contributing)
- [License](#license)
- [Contact](#contact)

---

## Features

- Configurable routes with regex-based path matching
- Include/exclude filters to refine route matching
- HTTP method filtering to exclude methods from caching
- In-memory response caching with TTL and max content size controls
- Customizable cache key generation
- Fallback proxy for unmatched requests
- Efficient HTTP handling using [fasthttp](https://github.com/valyala/fasthttp)
- Extensible logging with configurable output

---

## Getting Started

### Prerequisites

- Go 1.19 or later
- Basic familiarity with Go and YAML configuration files

### Installation

```bash
git clone https://github.com/yourorg/hermyx.git
cd hermyx
go build -o hermyx ./cmd/hermyx
````

### Configuration

Hermyx is configured via a YAML file. Example:

```yaml
server:
  port: 8080

cache:
  capacity: 1000          # Max cache entries
  ttl: 300000000000       # Cache TTL in nanoseconds (300s)
  maxContentSize: 1048576 # Max response size to cache (1MB)

log:
  toStdout: true
  prefix: "[Hermyx]"
  flags: 0

routes:
  - path: "^/api/.*"
    target: "http://localhost:9000"
    include:
      - "^/api/v1/.*"
    exclude:
      - "^/api/v1/private/.*"
    cache:
      enabled: true
      ttl: 600000000000  # 600s
      maxContentSize: 524288
      keyConfig:
        excludeMethods:
          - POST
```

### Running Hermyx

```bash
./hermyx -config /path/to/config.yaml
```

Starts the proxy on the configured port (default 8080).

---

## How It Works

1. Incoming requests are matched against configured routes using regex on the request path.
2. Routes apply include/exclude regex filters for fine-grained matching.
3. For matched routes, caching is applied to successful (2xx) responses based on TTL and max content size.
4. Cache keys are generated based on configurable rules involving request path, query, and method.
5. Requests that don’t match any route are proxied raw to the host specified by the `Host` header.

---

## Project Structure

```
engine/          # Core engine and proxy logic
pkg/cache/       # In-memory cache implementation
pkg/cachemanager/ # Cache management and key generation
pkg/models/      # Configuration and data models
pkg/utils/logger/ # Logger utility
pkg/utils/regex/  # Regex utilities
cmd/hermyx/      # CLI entry point
```

---

## Extending Hermyx

* Add new cache key generation strategies in `cachemanager`
* Implement custom route filters or matching
* Integrate advanced logging or metrics collection
* Customize fallback proxy behavior

---

## Contributing

Contributions, issues, and feature requests are welcome!
Feel free to check [issues page](https://github.com/spyder01/hermyx/issues).
Please follow the standard fork & pull request workflow.

---

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.

---

## Contact

For questions or support, open an issue or contact the maintainers.

---

*Happy proxying with Hermyx!* 🚀

```
