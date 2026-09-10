APP=cow-collector

.PHONY: build test run fmt

build:
	go build ./cmd/cow-collector

test:
	go test ./...

run:
	go run ./cmd/cow-collector

fmt:
	go fmt ./...
