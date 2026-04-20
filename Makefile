# Build shux
# Requires: mise with zig@0.15.2

ZIG=$(shell mise which zig 2>/dev/null || echo /tmp/fail)
PREFIX=$(shell pwd)/ghostty-build/usr

all:
	@if [ ! -f ghostty-build/usr/lib/libghostty-vt-static.a ]; then \
		mkdir -p ghostty-build && git clone --depth=1 https://github.com/ghostty-org/ghostty.git ghostty-build/ghostty; \
		cd ghostty-build/ghostty && $(ZIG) build -Doptimize=ReleaseFast -Demit-lib-vt=true; \
		mkdir -p $(PREFIX)/lib/pkgconfig $(PREFIX)/include; \
		cp zig-out/lib/libghostty-vt.a $(PREFIX)/lib/libghostty-vt-static.a; \
		cp -r include/ghostty $(PREFIX)/include/; \
		printf '%s\n' \
		  'prefix=$(PREFIX)' \
		  'exec_prefix=$${prefix}' \
		  'libdir=$${prefix}/lib' \
		  'includedir=$${prefix}/include' \
		  '' \
		  'Name: libghostty-vt-static' \
		  'Description: Ghostty VT static library' \
		  'Version: 0' \
		  'Libs: -L$${libdir} -lghostty-vt-static' \
		  'Cflags: -I$${includedir}' \
		  > $(PREFIX)/lib/pkgconfig/libghostty-vt-static.pc; \
	fi
	@PKG_CONFIG_PATH=$(PREFIX)/lib/pkgconfig go build -o shux ./cmd/shux && echo "✓ shux built"

clean:
	@rm -rf ghostty-build shux

test:
	@docker build -f Dockerfile.test -t shux-test . && docker run --rm shux-test

test-e2e: test
