BINARY  := leanreview
PKG     := ./cmd/leanreview
GOFLAGS := -trimpath

.PHONY: all build install test race vet fmt clean

all: build

## build: compile the binary into ./leanreview (default)
build:
	go build $(GOFLAGS) -o $(BINARY) $(PKG)

## install: install into GOBIN (or GOPATH/bin)
install:
	go install $(GOFLAGS) $(PKG)

## test: run the full test suite
test:
	go test ./...

## race: run the full test suite with the race detector
race:
	go test -race -count=1 ./...

## vet: run go vet across all packages
vet:
	go vet ./...

## fmt: gofmt all source in place
fmt:
	gofmt -w .

## clean: remove the built binary
clean:
	rm -f $(BINARY)
