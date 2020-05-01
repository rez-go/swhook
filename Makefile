
GOLANG_IMAGE ?= golang:1.14

.PHONY: all deps-up fmt linux-amd64

all: linux-amd64

fmt:
	@echo "Formatting files..."
	@docker run --rm \
		-v $(CURDIR):/workspace \
		--workdir /workspace \
		--entrypoint gofmt \
		$(GOLANG_IMAGE) -w -l -s \
		./

linux-amd64:
	@echo "Building for linux-amd64 ..."
	@export GOOS=linux
	@export GOARCH=amd64
	@docker run --rm \
		-v $(CURDIR):/workspace \
		--workdir /workspace \
		--entrypoint go \
		$(GOLANG_IMAGE) build -v -o build/swhook-linux-amd64 \
		.

# Update all dependencies
deps-up:
	@echo "Updating all dependencies..."
	@docker run --rm \
		-v $(CURDIR):/workspace \
		--workdir /workspace \
		$(GOLANG_IMAGE) /bin/sh -c "go get -u all && go mod tidy"
