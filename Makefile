# Build Ghostty’s VT lib into .deps/prefix, then `go build`. Override GHOSTTY_REF if needed.
# Usage: make | make go.  V=1 shows full commands.

MAKEFLAGS += --no-print-directory

GHOSTTY_REPO ?= https://github.com/ghostty-org/ghostty.git
GHOSTTY_REF  ?= v1.3.0

DEPS_DIR     := .deps
GHOSTTY_SRC  := $(DEPS_DIR)/ghostty
PREFIX       := $(abspath $(DEPS_DIR)/prefix)
PKG_CONFIG_PATH := $(PREFIX)/lib/pkgconfig:$(PREFIX)/share/pkgconfig

ZIG_FLAGS ?= --summary none

# Real libghostty: tests use -tags libghostty and cgo; requires make libghostty first.
GO_TEST_FLAGS := -count=1 -shuffle=off -tags libghostty
GO_TEST := CGO_ENABLED=1 PKG_CONFIG_PATH="$(PKG_CONFIG_PATH)" go test $(GO_TEST_FLAGS)

QUIET := @
ifeq ($(V),1)
  QUIET :=
endif

.PHONY: all help build go libghostty libghostty-clean \
	test test-sim test-integration test-e2e test-docker

all: build

help:
	@echo "make              — libghostty + go build"
	@echo "make go           — go build (needs .deps/prefix from libghostty)"
	@echo "make libghostty   — clone + zig build lib-vt"
	@echo "make libghostty-clean, V=1"
	@echo "make test         — sim + integration + e2e (go test, -tags libghostty; needs libghostty)"
	@echo "make test-sim, test-integration, test-e2e  (test/{sim,integration,e2e}/...)"
	@echo "make test-docker  — build dockerfile.sim.env, run make test, remove container (see --rm)"

build: libghostty go

$(GHOSTTY_SRC)/.git:
	$(QUIET)mkdir -p $(DEPS_DIR)
	@echo "Cloning Ghostty $(GHOSTTY_REF) (shallow)…"
	$(QUIET)git clone --depth 1 --branch $(GHOSTTY_REF) $(GHOSTTY_REPO) $(GHOSTTY_SRC)

libghostty: $(GHOSTTY_SRC)/.git
	$(QUIET)cd $(GHOSTTY_SRC) && zig build lib-vt -p $(PREFIX) -Doptimize=ReleaseFast $(ZIG_FLAGS)

libghostty-clean:
	rm -rf $(GHOSTTY_SRC) $(DEPS_DIR)/prefix

go:
	@test -d "$(PREFIX)/lib" || (echo "Missing $(PREFIX). Run: make libghostty" && exit 1)
	$(QUIET)PKG_CONFIG_PATH="$(PKG_CONFIG_PATH)" go build -o shux-dev .

test: test-sim test-integration test-e2e

test-sim: libghostty
	$(QUIET)$(GO_TEST) ./test/sim/...

test-integration: libghostty
	$(QUIET)$(GO_TEST) ./test/integration/...

test-e2e: libghostty
	$(QUIET)$(GO_TEST) ./test/e2e/...

test-docker:
	$(QUIET)docker build -f dockerfile.sim.env -t shux-test .
	$(QUIET)docker rm -f shux-test-run 2>/dev/null || true
	$(QUIET)docker run --rm --name shux-test-run shux-test
