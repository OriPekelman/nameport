.PHONY: all build clean test

BINARY_NAME=localhost-magic
DAEMON_NAME=localhost-magic-daemon
CLI_NAME=localhost-magic

all: build

build:
	go build -o $(DAEMON_NAME) ./cmd/daemon
	go build -o $(CLI_NAME) ./cmd/cli
	@echo "Built: $(DAEMON_NAME) and $(CLI_NAME)"

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(DAEMON_NAME)-linux ./cmd/daemon
	GOOS=linux GOARCH=amd64 go build -o $(CLI_NAME)-linux ./cmd/cli
	@echo "Built Linux binaries: $(DAEMON_NAME)-linux and $(CLI_NAME)-linux"

clean:
	rm -f $(DAEMON_NAME) $(CLI_NAME) $(DAEMON_NAME)-linux $(CLI_NAME)-linux

test:
	go test ./...

run: build
	sudo ./$(DAEMON_NAME)

dev: build
	./$(DAEMON_NAME)

fmt:
	go fmt ./...

lint:
	golangci-lint run

.DEFAULT_GOAL := build
