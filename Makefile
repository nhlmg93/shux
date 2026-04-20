# Makefile for gomux - handles Ghostty dependency
#
# Prerequisites:
#   - mise (https://mise.jdx.dev/) with zig@0.15.2
#
# Usage:
#   make         - Build gomux
#   make test    - Run tests
#   make clean   - Remove build artifacts

GHOSTTY_DIR = ghostty-build/ghostty
INSTALL_PREFIX = $(shell pwd)/ghostty-build/usr

# Single source of truth for Zig
ZIG = $(shell mise which zig 2>/dev/null)
ifeq ($(ZIG),)
  $(error "Zig not found. Run: mise use -g zig@0.15.2")
endif

all: gomux

clean:
	rm -rf ghostty-build gomux

$(GHOSTTY_DIR):
	@mkdir -p ghostty-build
	@git clone --depth=1 https://github.com/ghostty-org/ghostty.git $(GHOSTTY_DIR)

ghostty-build/usr/lib/libghostty-vt-static.a: $(GHOSTTY_DIR)
	@echo "Building Ghostty..."
	@cd $(GHOSTTY_DIR) && $(ZIG) build -Doptimize=ReleaseFast install
	@mkdir -p ghostty-build/usr/lib ghostty-build/usr/include
	@cp $(GHOSTTY_DIR)/zig-out/lib/libghostty-vt.a ghostty-build/usr/lib/libghostty-vt-static.a
	@cp -r $(GHOSTTY_DIR)/include/ghostty ghostty-build/usr/include/
	@echo "prefix=$(INSTALL_PREFIX)" > ghostty-build/usr/lib/pkgconfig/libghostty-vt-static.pc
	@echo "Libs: -L\$${prefix}/lib -lghostty-vt-static" >> ghostty-build/usr/lib/pkgconfig/libghostty-vt-static.pc
	@echo "Cflags: -I\$${prefix}/include" >> ghostty-build/usr/lib/pkgconfig/libghostty-vt-static.pc

gomux: ghostty-build/usr/lib/libghostty-vt-static.a
	@PKG_CONFIG_PATH=$(INSTALL_PREFIX)/lib/pkgconfig go build -o gomux ./cmd/gomux
	@echo "✓ gomux built"

test: ghostty-build/usr/lib/libghostty-vt-static.a
	@PKG_CONFIG_PATH=$(INSTALL_PREFIX)/lib/pkgconfig go test ./...

.PHONY: all clean test
