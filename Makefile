MAKEFLAGS += --no-print-directory

.DEFAULT_GOAL := build

GHOSTTY_REPO ?= https://github.com/ghostty-org/ghostty.git
GHOSTTY_REF  ?= main

DEPS_DIR := .deps
GHOSTTY_SRC := $(DEPS_DIR)/ghostty
PREFIX := $(abspath $(DEPS_DIR)/prefix)
PKG_CONFIG_PATH := $(PREFIX)/lib/pkgconfig:$(PREFIX)/share/pkgconfig

GO_TEST_FLAGS := -count=1 -shuffle=off
GO_TEST := CGO_ENABLED=1 PKG_CONFIG_PATH="$(PKG_CONFIG_PATH)" go test $(GO_TEST_FLAGS)

QUIET := @
ifeq ($(V),1)
  QUIET :=
endif

.PHONY: build test test-sim test-e2e test-integration libghostty clean help

build: libghostty
	$(QUIET)CGO_ENABLED=1 PKG_CONFIG_PATH="$(PKG_CONFIG_PATH)" go build -o shux .

help:
	@echo "make            — build shux"
	@echo "make build      — build shux"
	@echo "make test       — run sim and e2e tests"
	@echo "make test-sim   — run deterministic sim tests in Docker"
	@echo "make test-e2e   — run e2e tests locally"
	@echo "make test-integration — run integration tests locally"
	@echo "make clean      — remove local libghostty build"

test: test-sim test-e2e

test-sim:
	$(QUIET)docker build -f Dockerfile.sim.env -t shux-test .
	$(QUIET)docker rm -f shux-test-sim-run 2>/dev/null || true
	$(QUIET)docker run --rm --name shux-test-sim-run shux-test

test-e2e: libghostty
	$(QUIET)$(GO_TEST) ./test/e2e/...

test-integration: libghostty
	$(QUIET)$(GO_TEST) ./test/integration/...

$(GHOSTTY_SRC)/.git:
	$(QUIET)mkdir -p $(DEPS_DIR)
	@echo "Cloning Ghostty $(GHOSTTY_REF) (shallow)…"
	$(QUIET)git clone --depth 1 --branch $(GHOSTTY_REF) $(GHOSTTY_REPO) $(GHOSTTY_SRC)

libghostty: $(GHOSTTY_SRC)/.git
	$(QUIET)cd $(GHOSTTY_SRC) && zig build install -p $(PREFIX) -Doptimize=ReleaseFast -Demit-lib-vt=true --summary none

clean:
	$(QUIET)rm -rf $(GHOSTTY_SRC) $(DEPS_DIR)/prefix
