SHELL := /bin/bash

.PHONY: build dev test test-integration docker docker-run compose tidy fmt frontend backend

build: frontend backend ## Build static binary with embedded frontend

backend:
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o dist/cassidy ./cmd/server

frontend:
	cd web && pnpm install --frozen-lockfile && pnpm build

dev:
	cd web && pnpm dev &
	go run ./cmd/server

test:
	go test ./...

test-integration: ## Run cluster-touching tests against CASSANDRA_HOSTS (default 127.0.0.1:9042)
	go test -tags=integration ./...

tidy:
	go mod tidy

fmt:
	gofmt -s -w .

docker: ## Build the release image
	docker build -t cassidy:dev .

docker-run: docker ## Build + run the image with a persistent named volume
	docker run --rm -p 8080:8080 -v cassidy-data:/data cassidy:dev

compose: ## Bring up Cassidy + a throwaway Cassandra
	docker compose up --build
