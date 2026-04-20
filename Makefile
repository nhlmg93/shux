# Build gomux with Ghostty library

GHOSTTY_DIR = ghostty-build/ghostty
INSTALL_PREFIX = $(shell pwd)/ghostty-build/usr

ZIG = $(shell mise which zig 2>/dev/null)
ifeq ($(ZIG),)
  $(error "Zig not found. Run: mise use -g zig@0.15.2")
endif

all:
	@if [ ! -f ghostty-build/usr/lib/libghostty-vt-static.a ]; then \
		echo "Building Ghostty library..."; \
		mkdir -p ghostty-build; \
		git clone --depth=1 https://github.com/ghostty-org/ghostty.git $(GHOSTTY_DIR); \
		cd $(GHOSTTY_DIR) && $(ZIG) build -Doptimize=ReleaseFast -Demit-lib-vt=true; \
		mkdir -p ghostty-build/usr/lib ghostty-build/usr/include ghostty-build/usr/lib/pkgconfig; \
		cp $(GHOSTTY_DIR)/zig-out/lib/libghostty-vt.a ghostty-build/usr/lib/libghostty-vt-static.a; \
		cp -r $(GHOSTTY_DIR)/include/ghostty ghostty-build/usr/include/; \
		echo "prefix=$(INSTALL_PREFIX)" > ghostty-build/usr/lib/pkgconfig/libghostty-vt-static.pc; \
		echo "Libs: -L\$${prefix}/lib -lghostty-vt-static" >> ghostty-build/usr/lib/pkgconfig/libghostty-vt-static.pc; \
		echo "Cflags: -I\$${prefix}/include" >> ghostty-build/usr/lib/pkgconfig/libghostty-vt-static.pc; \
	fi
	@PKG_CONFIG_PATH=$(INSTALL_PREFIX)/lib/pkgconfig go build -o gomux ./cmd/gomux
	@echo "✓ gomux built"

clean:
	rm -rf ghostty-build gomux

test: all
	@PKG_CONFIG_PATH=$(INSTALL_PREFIX)/lib/pkgconfig go test ./...
