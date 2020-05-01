
GOLANG_IMAGE ?= golang:1.14

.PHONY: all deps-up fmt linux-amd64

all: linux-amd64

fmt:
	@echo "Formatting files..."
	@gofmt -w -l -s ./

linux-amd64:
	@echo "Building for linux-amd64 ..."
	@export GOOS=linux
	@export GOARCH=amd64
	@go build -v -o build/swhook-linux-amd64 .

# Update all dependencies
deps-up:
	@echo "Updating all dependencies..."
	@/bin/sh -c "go get -u all && go mod tidy"
