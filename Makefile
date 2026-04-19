# Makefile for gomux - handles Ghostty dependency

GHOSTTY_COMMIT ?= main
GHOSTTY_VENDOR_DIR ?= ghostty-build
INSTALL_PREFIX ?= $(shell pwd)/$(GHOSTTY_VENDOR_DIR)/usr
ZIG_VERSION ?= 0.14.0
ZIG_DIR ?= $(GHOSTTY_VENDOR_DIR)/zig

all: check-zig libghostty-vt-static.a gomux

check-zig:
	@if ! command -v zig >/dev/null 2>&1 && [ ! -f $(ZIG_DIR)/zig ]; then \
		echo "Zig not found. Downloading..."; \
		mkdir -p $(ZIG_DIR); \
		curl -L "https://ziglang.org/download/$(ZIG_VERSION)/zig-linux-x86_64-$(ZIG_VERSION).tar.xz" | tar -xJ --strip-components=1 -C $(ZIG_DIR); \
	fi

ZIG := $(shell if [ -f $(ZIG_DIR)/zig ]; then echo $(ZIG_DIR)/zig; else echo zig; fi)

clean:
	rm -rf $(GHOSTTY_VENDOR_DIR)
	rm -f gomux

go-clean:
	go clean

# Build the static library from Ghostty source
libghostty-vt-static.a: $(GHOSTTY_VENDOR_DIR)/ghostty
	@echo "Building Ghostty static library..."
	cd $(GHOSTTY_VENDOR_DIR)/ghostty && \
		$(ZIG) build -Doptimize=ReleaseFast -Dcpu=baseline libghostty
	cp $(GHOSTTY_VENDOR_DIR)/ghostty/zig-out/lib/libghostty-vt-static.a .
	@echo "✓ Ghostty library built"

# Clone Ghostty source
$(GHOSTTY_VENDOR_DIR)/ghostty:
	@echo "Cloning Ghostty..."
	mkdir -p $(GHOSTTY_VENDOR_DIR)
	git clone https://github.com/ghostty-org/ghostty.git --depth=1 $(GHOSTTY_VENDOR_DIR)/ghostty
	cd $(GHOSTTY_VENDOR_DIR)/ghostty && git reset --hard && git clean -fdx
	if [ -n "$(GHOSTTY_COMMIT)" ]; then \
		cd $(GHOSTTY_VENDOR_DIR)/ghostty && git fetch --depth=1 origin $(GHOSTTY_COMMIT) && git checkout FETCH_HEAD; \
	fi

# Build gomux binary
# Requires PKG_CONFIG_PATH to find libghostty
gomux: libghostty-vt-static.a
	@echo "Building gomux..."
	export PKG_CONFIG_PATH=$(INSTALL_PREFIX)/lib/pkgconfig:$$PKG_CONFIG_PATH && \
		go build -o gomux ./cmd/gomux
	@echo "✓ gomux built"

# Run tests
test: libghostty-vt-static.a
	export PKG_CONFIG_PATH=$(INSTALL_PREFIX)/lib/pkgconfig:$$PKG_CONFIG_PATH && \
		go test -v ./...

# Install locally (for development)
install: libghostty-vt-static.a
	@echo "Installing to $(INSTALL_PREFIX)..."
	mkdir -p $(INSTALL_PREFIX)/lib/pkgconfig $(INSTALL_PREFIX)/include
	cp libghostty-vt-static.a $(INSTALL_PREFIX)/lib/
	cp -r $(GHOSTTY_VENDOR_DIR)/ghostty/include/ghostty $(INSTALL_PREFIX)/include/
	@echo "Creating pkg-config file..."
	@echo "prefix=$(INSTALL_PREFIX)" > $(INSTALL_PREFIX)/lib/pkgconfig/libghostty-vt-static.pc
	@echo "exec_prefix=\$${prefix}" >> $(INSTALL_PREFIX)/lib/pkgconfig/libghostty-vt-static.pc
	@echo "libdir=\$${prefix}/lib" >> $(INSTALL_PREFIX)/lib/pkgconfig/libghostty-vt-static.pc
	@echo "includedir=\$${prefix}/include" >> $(INSTALL_PREFIX)/lib/pkgconfig/libghostty-vt-static.pc
	@echo "" >> $(INSTALL_PREFIX)/lib/pkgconfig/libghostty-vt-static.pc
	@echo "Name: libghostty-vt-static" >> $(INSTALL_PREFIX)/lib/pkgconfig/libghostty-vt-static.pc
	@echo "Description: Ghostty terminal emulator library (static)" >> $(INSTALL_PREFIX)/lib/pkgconfig/libghostty-vt-static.pc
	@echo "Version: 1.0.0" >> $(INSTALL_PREFIX)/lib/pkgconfig/libghostty-vt-static.pc
	@echo "Libs: -L\$${libdir} -lghostty-vt-static" >> $(INSTALL_PREFIX)/lib/pkgconfig/libghostty-vt-static.pc
	@echo "Cflags: -I\$${includedir}" >> $(INSTALL_PREFIX)/lib/pkgconfig/libghostty-vt-static.pc
	@echo "✓ Installed to $(INSTALL_PREFIX)"

.PHONY: all clean go-clean test install check-zig
