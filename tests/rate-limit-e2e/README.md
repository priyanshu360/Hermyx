# Rate Limiter E2E Tests

This directory contains comprehensive end-to-end tests for Hermyx rate limiting functionality using both memory and Redis storage backends.

## Overview

The E2E tests validate various rate limiting scenarios including:
- **Strict Rate Limiting**: 2 requests per minute
- **Moderate Rate Limiting**: 5 requests per 30 seconds  
- **Header-based Rate Limiting**: 3 requests per minute by User-Agent
- **Combined Key Rate Limiting**: 2 requests per minute by IP + User-Agent
- **Burst Protection**: 8 requests per 10 seconds
- **Global Rate Limiting**: 10 requests per minute (inherited)
- **No Rate Limiting**: Disabled rate limits
- **Redis Persistence**: Rate limits persist across restarts

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Test Client   │───▶│   Hermyx        │───▶│  Mock Server    │
│   (curl)        │    │   (Port 8080)   │    │  (Port 8081)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │   Redis         │
                       │   (Port 6379)   │
                       └─────────────────┘
```

## Prerequisites

- **Docker** (20.10+)
- **Docker Compose** (2.0+)
- **Go** (1.19+)
- **curl** (for health checks)
- **bash** (4.0+)

## Quick Start

### 1. Setup Environment

```bash
# Navigate to the e2e test directory
cd tests/rate-limit-e2e

# Run the setup script
./setup.sh
```

This will:
- Check prerequisites
- Start Docker services (Redis, Mock Server)
- Wait for services to be ready
- Verify all services are responding

### 2. Run Tests

```bash
# Run Memory Rate Limiter tests
./memory-rate-limit-e2e-tests.sh

# Run Redis Rate Limiter tests  
./redis-rate-limiter-e2e-tests.sh

```

### 3. Cleanup

```bash
# Stop and remove all containers
./teardown.sh
```

## Test Configuration

### Memory Rate Limiter (`configs/memory-rate-limiter.yml`)

- **Storage**: Memory-based rate limiting
- **Global Limit**: 10 requests per minute
- **Routes**: 7 different API endpoints with various rate limiting rules
- **Cache**: Disabled (focus on rate limiting)

### Redis Rate Limiter (`configs/redis-rate-limiter.yml`)

- **Storage**: Redis-based rate limiting
- **Global Limit**: 10 requests per minute  
- **Routes**: Same 7 API endpoints as memory config
- **Persistence**: Rate limits persist across restarts
- **Cache**: Disabled (focus on rate limiting)

## Test Scenarios

### 1. Strict API (`/get`)
- **Limit**: 2 requests per minute
- **Key**: IP address
- **Test**: Verify rate limiting after 2 requests

### 2. Moderate API (`/json`)
- **Limit**: 5 requests per 30 seconds
- **Key**: IP address
- **Test**: Verify rate limiting after 5 requests

### 3. Header API (`/headers`)
- **Limit**: 3 requests per minute
- **Key**: User-Agent header
- **Test**: Verify different User-Agents have separate limits

### 4. Combined API (`/user-agent`)
- **Limit**: 2 requests per minute
- **Key**: IP + User-Agent combination
- **Test**: Verify combined key rate limiting

### 5. Burst API (`/ip`)
- **Limit**: 8 requests per 10 seconds
- **Key**: IP address
- **Test**: Verify burst protection

### 6. Global API (`/uuid`)
- **Limit**: 10 requests per minute (inherited)
- **Key**: IP address
- **Test**: Verify global rate limiting

### 7. No Limit API (`/status/200`)
- **Limit**: Disabled
- **Test**: Verify no rate limiting applied

## Service Endpoints

| Service | URL | Description |
|---------|-----|-------------|
| Redis | `localhost:6379` | Rate limiting storage |
| Mock Server | `localhost:8081` | Backend API for testing |
| Hermyx | `localhost:8080` | Rate limiter (memory or Redis) |

## Test Results

Test results are displayed in the terminal and logged to:
- `logs/hermyx-memory.log` - Memory rate limiter logs
- `logs/hermyx-redis.log` - Redis rate limiter logs

## Manual Testing

You can manually test the rate limiters:

```bash
# Test Memory Rate Limiter
curl http://localhost:8080/get
curl http://localhost:8080/json
curl http://localhost:8080/headers -H "User-Agent: Test Browser"

# Test Redis Rate Limiter (after running Redis tests)
curl http://localhost:8080/get
curl http://localhost:8080/json
curl http://localhost:8080/headers -H "User-Agent: Test Browser"

# Check rate limit headers
curl -I http://localhost:8080/get
```

## Troubleshooting

### Services Not Starting

```bash
# Check Docker status
docker ps -a

# Check logs
docker-compose logs redis
docker-compose logs mock-server

# Restart services
docker-compose restart
```

### Rate Limiting Not Working

1. **Check Configuration**: Verify config files are correct
2. **Check Logs**: Look for errors in Hermyx logs (`logs/hermyx-*.log`)
3. **Check Redis**: Ensure Redis is accessible and responding
4. **Check Routes**: Verify route patterns match test URLs
5. **Check Build**: Ensure Hermyx builds successfully

### Test Failures

1. **Check Service Health**: Ensure all services are responding
2. **Check Network**: Verify containers can communicate
3. **Check Timing**: Some tests require specific timing
4. **Check Logs**: Review test output for errors
5. **Check Ports**: Ensure port 8080 is available

### Common Issues

#### Port Conflicts
```bash
# Check if ports are in use
netstat -tulpn | grep :8080
netstat -tulpn | grep :8081
netstat -tulpn | grep :6379

# Kill processes on port 8080
lsof -ti:8080 | xargs kill -9
fuser -k 8080/tcp
```

#### Docker Issues
```bash
# Clean up Docker resources
docker system prune -a --volumes

# Rebuild services
docker-compose build --no-cache
```

#### Permission Issues
```bash
# Make scripts executable
chmod +x *.sh

# Check Docker permissions
sudo usermod -aG docker $USER
```

## Development

### Adding New Tests

1. **Update Config**: Add new routes to config files
2. **Update Scripts**: Add test functions to test scripts
3. **Test**: Run tests to verify functionality

### Modifying Rate Limits

1. **Edit Config**: Update rate limiting rules in config files
2. **Restart Tests**: Run test scripts to apply changes
3. **Run Tests**: Verify changes work correctly

### Debugging

```bash
# Monitor logs in real-time
tail -f logs/hermyx-memory.log
tail -f logs/hermyx-redis.log

# Check Redis data
docker exec -it redis redis-cli
> KEYS *
> GET <key>
```

## Performance Testing

For performance testing, you can modify the test scripts to:
- Increase request rates
- Test with multiple concurrent clients
- Measure response times
- Test under load

## Contributing

When adding new test scenarios:
1. Update both memory and Redis configs
2. Add corresponding test functions
3. Update this README
4. Test thoroughly before committing
5. Ensure tests pass for both memory and Redis backends

## License

This E2E test suite is part of the Hermyx project and follows the same license terms.
