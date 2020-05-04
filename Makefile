
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
	@docker run --rm \
			-v $(CURDIR):/workspace \
			-e GOOS=linux \
			-e GOARCH=amd64 \
			--workdir /workspace \
			--entrypoint go \
			$(GOLANG_IMAGE) build -o build/swhook-linux-amd64 \
			-ldflags="-s -w -X main.revisionID=$$(git rev-parse HEAD) -X main.buildTimestamp=$$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
			.

# Update all dependencies
deps-up:
	@echo "Updating all dependencies..."
	@docker run --rm \
		-v $(CURDIR):/workspace \
		--workdir /workspace \
		$(GOLANG_IMAGE) /bin/sh -c "go get -u all && go mod tidy"
