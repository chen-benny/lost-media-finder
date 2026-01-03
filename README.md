# Lost Media Finder
Distributed Go web crawler to find the lost media: gfap at vidlii.com.

## Quick Start
```bash
# Start infrastructure
make docker-up

# Run crawler
make crawler

# View status
make status
```

## Requirements

- Go 1.21+
- Docker & Docker Compose
- 2GB RAM minimum

## Configuration
Environment variables:
```
WORKERS=4           # Number of concurrent workers
RATE_LIMIT=2        # Requests per second
REDIS_ADDR=localhost:6379
MONGO_URI=mongodb://localhost:27017
```

## Commands
```
make docker-up      # Start Redis/Mongo/Prometheus/Grafana
make docker-down    # Stop services
make docker-logs    # View logs
make crawler        # Run crawler
make status         # Check status
make clean          # Remove all data
```
