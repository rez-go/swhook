
GOLANG_IMAGE ?= golang:1.14

.PHONY: all deps-up fmt linux-amd64 darwin-amd64

all: linux-amd64 darwin-amd64

fmt:
	@echo "Formatting files..."
	@gofmt -w -l -s ./

linux-amd64:
	@echo "Building for linux-amd64 ..."
	@export GOOS=linux; \
		export GOARCH=amd64; \
		go build -o build/swhook-linux-amd64 \
		-ldflags="-s -w -X main.revisionID=$$(git rev-parse HEAD) -X main.buildTimestamp=$$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
		.

darwin-amd64:
	@echo "Building for darwin-amd64 ..."
	@export GOOS=darwin; \
		export GOARCH=amd64; \
		go build -o build/swhook-darwin-amd64 \
		-ldflags="-s -w -X main.revisionID=$$(git rev-parse HEAD) -X main.buildTimestamp=$$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
		.

# Update all dependencies
deps-up:
	@echo "Updating all dependencies..."
	@go get -u all
	@go mod tidy
