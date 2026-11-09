VERSION ?= 0.1.0
GO      ?= go
LDFLAGS := -s -w -X devhive-tunnel/pkg/conf.Version=$(VERSION)

.PHONY: all build hub devtun mcp test e2e dist run-hub clean

all: build

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/devhive-hub ./cmd/devhive-hub
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/devtun ./cmd/devtun
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/dh-mcp ./cmd/dh-mcp

hub: build
devtun: build
mcp: build

test:
	$(GO) test ./...

e2e:
	$(GO) test ./pkg/hub -run TestEndToEnd -v

dist:
	scripts/build-all.sh $(VERSION)

run-hub:
	$(GO) run ./cmd/devhive-hub

clean:
	rm -rf bin dist
