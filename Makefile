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
		echo "prefix=$(PREFIX)\nLibs: -L\$${prefix}/lib -lghostty-vt-static\nCflags: -I\$${prefix}/include" > $(PREFIX)/lib/pkgconfig/libghostty-vt-static.pc; \
	fi
	@PKG_CONFIG_PATH=$(PREFIX)/lib/pkgconfig go build -o shux ./cmd/shux && echo "✓ shux built"

clean:
	@rm -rf ghostty-build shux

test: all
	@PKG_CONFIG_PATH=$(PREFIX)/lib/pkgconfig go test -v ./pkg/...

test-e2e:
	@docker build -f Dockerfile.test -t shux-test . && docker run --rm shux-test
