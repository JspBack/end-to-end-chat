.PHONY: build run test clean pre-all configure-pre-commit

all: clean test build

build:
	go build -ldflags="-s -w" -o bin_client/client ./cmd/
	go build -ldflags="-s -w" -o bin_peer/peer ./cmd/

test:
	go test ./app_test

clean:
	rm -rf bin*

pre-all:
	pre-commit run --all-files

configure-pre-commit:
	pip install pre-commit
	pre-commit install
	pre-commit autoupdate
