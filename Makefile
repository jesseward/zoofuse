.PHONY: all fmt check-fmt build test clean

all: check-fmt build test

fmt:
	go fmt ./...

check-fmt:
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "Go code is not formatted. Run 'make fmt' to fix."; \
		gofmt -l .; \
		exit 1; \
	fi

build:
	go build -v ./...

test:
	go test -v -count=1 ./...

clean:
	go clean
