---
title: Install and build
description: Build shux from source on Linux.
---

shux is built from source. You need Go, a C toolchain, Zig, and pkg-config to compile libghostty.

## Prerequisites

On Arch Linux or Debian/Ubuntu:

```bash
# Debian/Ubuntu
sudo apt-get install make git zig pkg-config gcc g++ libc6-dev

# Arch
sudo pacman -S make git zig pkg-config gcc
```

You also need [Go](https://go.dev/dl/) matching the version in `go.mod`.

## Build

Clone the repository and run:

```bash
git clone https://github.com/nhlmg93/shux.git
cd shux
make
```

The first build compiles libghostty from source (this can take a few minutes). The resulting binary is `./shux` in the repository root.

## Verify

```bash
./shux --version
```

## Optional config

Copy the example config into your XDG config directory:

```bash
mkdir -p ~/.config/shux
cp -r runtime/example/* ~/.config/shux/
```

See [Configuration overview](/configuration/overview/) for details.
