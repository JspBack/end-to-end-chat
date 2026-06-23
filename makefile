.PHONY: build run test clean pre-all configure-pre-commit

all: clean test build run

build:
	go build -ldflags="-s -w" -o bin_client/client ./cmd/
	go build -ldflags="-s -w" -o bin_peer/peer ./cmd/

run:
	./bin_client/client

test:
	go test ./test

clean:
	rm -rf bin*
	rm -rf data/

pre-all:
	pre-commit run --all-files

configure-pre-commit:
	pip install pre-commit
	pre-commit install
	pre-commit autoupdate
