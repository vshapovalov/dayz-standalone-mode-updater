.PHONY: fmt lint test run

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')

lint:
	golangci-lint run ./...

test:
	go test ./...

run:
	go run ./cmd/dayzmods run --config config.json
