.PHONY: run build test lint tidy docker-up

run:
	go run ./main.go

build:
	go build -o bin/server ./main.go

test:
	go test ./... -v

tidy:
	go mod tidy

fmt:
	go fmt ./...
