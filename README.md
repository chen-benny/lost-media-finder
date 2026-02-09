markdown

# Lost Media Finder
Concurrent Go web crawler for finding lost Japanese videos on VidLii.com (>10min, uploaded before 2022).

## Quick Start
```bash
make docker    # Start services
make run       # Build and run crawler
make status    # Check status
```

## Requirements
- Go 1.25+
- Docker & Docker Compose
- 2GB RAM minimum

## Commands
```bash
make docker      # Start Redis/MongoDB/Prometheus
make stop        # Stop Docker services
make logs        # View Docker logs
make build       # Build crawler binary
make run         # Run crawler in background
make test        # Run test mode (10 videos)
make status      # Show service status
make restart     # Restart crawler
make clean       # Delete all data
```

## Monitoring
- **Prometheus**: http://localhost:9090
- **Metrics**: http://localhost:2112/metrics

## Architecture
```
cmd/crawler/        - Entry point
internal/
  ├── config/       - Configuration
  ├── crawler/      - Core crawling logic
  ├── storage/      - Redis & MongoDB clients
  ├── metrics/      - Prometheus metrics
  ├── model/        - Data models
  └── auth/         - HTTP client
```

## VPS Deployment
```bash
# Install dependencies
sudo apt update
sudo apt install -y docker.io docker-compose golang-go git make

# Clone and deploy
git clone 
cd lost-media-finder
make docker
make run

# Monitor
make status
tail -f crawler.log
```

## Features
- Auto-resume from last state
- Redis deduplication
- MongoDB persistence
- Prometheus metrics
- Rate limiting (2 req/sec)
- Retry logic with exponential backoff