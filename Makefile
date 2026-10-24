GO ?= go
VERSION ?= dev

.PHONY: build build-linux-amd64 test check release clean
build:
	mkdir -p bin
	$(GO) build -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o bin/ota-headend ./cmd/ota-headend

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(MAKE) build

test:
	$(GO) test -race ./...

check:
	./scripts/check.sh

release:
	./scripts/release.sh $(VERSION)

clean:
	$(GO) clean
