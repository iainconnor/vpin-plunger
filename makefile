BINARY_NAME=vpin
BUILD_DIR=./dist
GO=go
GOFLAGS=-trimpath

.PHONY: build run test lint tidy clean release snapshot

build:
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/plunger
run:
	$(GO) run ./cmd/plunger
test:
	$(GO) test ./... -v -race
lint:
	golangci-lint run ./...
tidy:
	$(GO) mod tidy
clean:
	rm -rf $(BUILD_DIR)
release:
	goreleaser release --clean
snapshot:
	goreleaser release --snapshot --clean
