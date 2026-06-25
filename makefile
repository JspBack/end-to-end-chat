.PHONY: all build test clean pre-all configure-pre-commit

all: clean test build

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-s -w -X github.com/JspBack/end-to-end-chat/config.Version=$(VERSION)"

build:
	mkdir -p bin_client bin_peer
	go build $(LDFLAGS) -o bin_client/client ./cmd/
	cp bin_client/client bin_peer/peer

test:
	go test ./test

clean:
	rm -rf bin*

pre-all:
	pre-commit run --all-files

configure-pre-commit:
	pip install pre-commit
	pre-commit install
	pre-commit autoupdate
