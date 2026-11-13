.PHONY: build test clean

BINARY := bin/notirex

build:
	mkdir -p bin
	go build -o $(BINARY) ./cmd/notirex

test:
	go test ./...

clean:
	rm -rf bin
