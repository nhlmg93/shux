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

QUIET := @
ifeq ($(V),1)
  QUIET :=
endif

.PHONY: all help build go libghostty libghostty-clean

all: build

help:
	@echo "make              — libghostty + go build"
	@echo "make go           — go build (needs .deps/prefix from libghostty)"
	@echo "make libghostty   — clone + zig build lib-vt"
	@echo "make libghostty-clean, V=1"

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
