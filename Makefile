# Build shux
# Requires: zig 0.15.2

PREFIX=$(shell pwd)/ghostty-build/usr
GHOSTTY_REPO ?= https://github.com/ghostty-org/ghostty.git
GHOSTTY_REF ?= dcc39dcd401975ee77a642fa15ba7bb9f6d85b96

all: ghostty-vt
	@PKG_CONFIG_PATH=$(PREFIX)/lib/pkgconfig go build -o shux ./cmd/shux && echo "✓ shux built"

ghostty-vt:
	@GHOSTTY_REPO=$(GHOSTTY_REPO) GHOSTTY_REF=$(GHOSTTY_REF) ./scripts/build-ghostty-vt.sh $(PREFIX)

clean:
	@rm -rf ghostty-build shux

test:
	@docker build --build-arg GHOSTTY_REF=$(GHOSTTY_REF) -f Dockerfile.test -t shux-test . && docker run --rm shux-test

test-native: ghostty-vt
	@SHUX_E2E=1 PKG_CONFIG_PATH=$(PREFIX)/lib/pkgconfig go test ./... -v -count=1

test-ci: test-native

test-e2e: test
