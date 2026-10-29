.PHONY: build test lint cover run generate

build:
	go build -o bin/lastro ./cmd/lastro

test:
	go test -race ./...

lint:
	golangci-lint run

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

run:
	go run ./cmd/lastro serve

generate:
	go tool sqlc generate
	go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml
