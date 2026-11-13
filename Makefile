.PHONY: build test clean

BINARY := bin/notyrex

build:
	mkdir -p bin
	go build -o $(BINARY) ./cmd/notyrex

test:
	go test ./...

clean:
	rm -rf bin
