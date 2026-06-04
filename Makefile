.PHONY: run test build fmt lint docker-build docker-up docker-down docker-logs migrate dev docs help

run:
	go run ./cmd/hub serve

test:
	go test ./...

build:
	go build -o bin/hub ./cmd/hub

fmt:
	go fmt ./...

lint:
	golangci-lint run

docker-build:
	docker build -t hub .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f hub

migrate:
	go run ./cmd/hub migrate

dev:
	go run ./cmd/hub serve

docs:
	go run ./cmd/openapi

help:
	@printf 'Targets: run test build fmt lint docker-build docker-up docker-down docker-logs migrate dev docs\n'
