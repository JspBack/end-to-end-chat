.PHONY: build run test clean pre-all configure-pre-commit

all: clean test build run

build:
	go build -ldflags="-s -w" -o bin/client ./cmd/
	go build -ldflags="-s -w" -o bin/peer ./cmd/peer/

run:
	./bin/client

test:
	go test ./test

clean:
	rm -rf bin/
	rm -rf data/

pre-all:
	pre-commit run --all-files

configure-pre-commit:
	pip install pre-commit
	pre-commit install
	pre-commit autoupdate
