# Build shux
# Requires: zig 0.15.2

PREFIX=$(shell pwd)/ghostty-build/usr
GHOSTTY_REPO ?= https://github.com/ghostty-org/ghostty.git
GHOSTTY_REF ?= dcc39dcd401975ee77a642fa15ba7bb9f6d85b96
GOFILES = $(shell find . -name '*.go' -not -path './ghostty-build/*')
GO_WRAP = $(shell if command -v mise >/dev/null 2>&1; then printf 'mise exec go -- '; fi)
GO = $(GO_WRAP)go
GOPLS = $(GO_WRAP)gopls
GOFUMPT = $(GO_WRAP)gofumpt
GOLANGCI_LINT = $(GO_WRAP)golangci-lint

.PHONY: all build ghostty-vt clean test test-native ci-test test-ci doc gopls fmt fmt-check lint check

all: build

build: ghostty-vt
	@PKG_CONFIG_PATH=$(PREFIX)/lib/pkgconfig $(GO) build -o shux ./cmd/shux && echo "✓ shux built"

ghostty-vt:
	@GHOSTTY_REPO=$(GHOSTTY_REPO) GHOSTTY_REF=$(GHOSTTY_REF) ./scripts/build-ghostty-vt.sh $(PREFIX)

clean:
	@rm -rf ghostty-build shux

test: ghostty-vt
	@PKG_CONFIG_PATH=$(PREFIX)/lib/pkgconfig $(GO) test ./... -v -count=1

test-native: test

ci-test:
	@docker build --build-arg GHOSTTY_REF=$(GHOSTTY_REF) -f Dockerfile.test -t shux-test . && docker run --rm shux-test

test-ci: check test ci-test

doc:
	@$(GO) doc ./pkg/shux >/dev/null
	@echo "✓ docs"

gopls:
	@$(GOPLS) check $(GOFILES)

fmt:
	@$(GOFUMPT) -w .

fmt-check:
	@out="$$($(GOFUMPT) -l .)"; \
	if [ -n "$$out" ]; then \
		echo "Files need formatting:"; \
		echo "$$out"; \
		exit 1; \
	fi

lint:
	@$(GOLANGCI_LINT) run ./...

check: fmt-check gopls lint

test-e2e: test
