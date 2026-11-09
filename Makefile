SHELL := /bin/sh

BINARY  := bin/stream-bridge
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildDate=$(DATE)

.PHONY: all build test test-unit test-chaos test-integration lint fmt vet coverage clean docker compose-up compose-down

all: lint test build

build:
	mkdir -p $(dir $(BINARY))
	CGO_ENABLED=1 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

test: test-unit

test-unit:
	go test -race ./...

test-chaos:
	go test -race -tags chaos ./test/chaos

test-integration:
	./scripts/test-integration.sh

lint:
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run

fmt:
	gofmt -w .

vet:
	go vet ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

clean:
	rm -rf bin coverage.out coverage.html

docker:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(DATE) \
		-t go-stream-bridge:$(VERSION) .

compose-up:
	docker compose -f docker-compose.test.yml --profile core up -d --wait

compose-down:
	docker compose -f docker-compose.test.yml --profile core down -v
