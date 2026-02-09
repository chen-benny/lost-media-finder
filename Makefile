.PHONY: help docker stop logs build run test status restart clean

help:
	@echo "VidLii Crawler - commands:"
	@echo "  make docker      - Start Docker services (Redis/MongoDB/Prometheus)"
	@echo "  make stop        - Stop all Docker services"
	@echo "  make logs        - View Docker logs"
	@echo "  make build       - Build crawler binary"
	@echo "  make run         - Run crawler in background"
	@echo "  make test        - Run crawler in test mode"
	@echo "  make status      - Show service status"
	@echo "  make restart     - Restart crawler"
	@echo "  make clean       - Stop everything and remove data"

docker:
	docker-compose up -d
	@echo ""
	@echo "Services started:"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Metrics:    http://localhost:2112/metrics"

stop:
	docker-compose down

logs:
	docker-compose logs -f

build:
	go build -o crawler cmd/crawler/main.go
	@echo "Built: ./crawler"

run: build
	-@pkill -x "crawler"
	@nohup ./crawler > /dev/null 2>&1 & echo "Crawler started in background"

test:
	go run cmd/crawler/main.go --test

restart:
	-@pkill -x "crawler"
	@sleep 1
	@nohup ./crawler > /dev/null 2>&1 &
	@echo "Crawler restarted"

status:
	@echo "=== Docker Services ==="
	@docker-compose ps
	@echo ""
	@echo "=== Crawler Status ==="
	@pgrep -x "crawler" > /dev/null && echo "Crawler: Running (PID: $$(pgrep -x "crawler"))" || echo "Crawler: Not running"
	@echo ""
	@echo "=== Metrics ==="
	@curl -s http://localhost:2112/metrics | grep "crawler_" | head -5 || echo "Metrics not available"

clean:
	@echo "WARNING: This will delete all data!"
	-@pkill -f "./crawler"
	-docker exec lost-media-finder-redis-1 redis-cli FLUSHALL
	docker-compose down -v
	rm -f crawler.log targets.json
	@echo "All data deleted"
