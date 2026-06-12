BINARY_NAME := nydus-convert
CMD_DIR := ./cmd/nydus-convert
BUILD_DIR := build
BIN_PATH := $(BUILD_DIR)/$(BINARY_NAME)

.PHONY: all build test fmt tidy clean run help

all: fmt test build

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BIN_PATH) $(CMD_DIR)

test:
	go test ./...

fmt:
	gofmt -w cmd internal

tidy:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)

run:
	go run $(CMD_DIR) run $(ARGS)

help:
	@printf "Available targets:\n"
	@printf "  make build  Build CLI binary to %s\n" "$(BIN_PATH)"
	@printf "  make test   Run Go tests\n"
	@printf "  make fmt    Format Go source files\n"
	@printf "  make tidy   Tidy Go modules\n"
	@printf "  make clean  Remove build outputs\n"
	@printf "  make run ARGS='...'  Run CLI with arguments\n"
	@printf "  make all    Run fmt, test, and build\n"
