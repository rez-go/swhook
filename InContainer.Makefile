
GOLANG_IMAGE ?= golang:1.14

.PHONY: fmt linux-amd64

fmt:
	@echo "Formatting files..."
	@gofmt -w -l -s ./

linux-amd64:
	@echo "Building for linux-amd64 ..."
	@export GOOS=linux
	@export GOARCH=amd64
	@go build -v -o build/swhook-linux-amd64 .
