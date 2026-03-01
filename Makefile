.PHONY: build test vet lint clean

BINARY := bin/insta360ctl

build:
	go build -o $(BINARY) ./cmd/insta360ctl/

test:
	go test -race ./...

vet:
	go vet ./...

lint: vet
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"

clean:
	rm -f $(BINARY)

all: build test vet
