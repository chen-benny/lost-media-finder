.PHONY: help docker-up docker-down docker-logs crawler status clean

help:
	@echo "lost-media-finder commands:"
	@echo " make docker-up		- Start Redis/MongoDB/Prometheus/Grafana"
	@echo " make docker-down	- Stop all Docker services"
	@echo " make docker-logs	- View Docker logs"
	@echo " make crawler		- Run crawler"
	@echo " make status			- Show service status"
	@echo " make clean			- Stop everything and remove data"

docker-up:
	docker-compose up -d
	@echo "Services started:"
	@echo " Grafana:		http://localhost:3000"
	@echo " Prometheus:		http://localhost:9090"

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

crawler:
	go run cmd/crawler/main.go

status:
	@docker-compose ps
	@echo ""
	@curl -s http://localhost:2112/metrics | grep crawler || echo "Crawler not running"

clean:
	docker-compose down -v
	@echo "All data deleted"