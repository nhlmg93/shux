# gomux-term - Alacritty FFI Wrapper

Rust FFI bindings exposing Alacritty's terminal emulator to Go.

## Why

Instead of reimplementing terminal escape sequences, colors, Unicode handling,
and scrollback in Go, we use Alacritty's battle-tested `alacritty_terminal`
crate via FFI.

## Architecture

```
Go (bubbletea UI)
    ↕ FFI (Cgo)
Rust (gomux-term)
    ↕ uses
Alacritty Terminal (grid, parser, PTY)
    ↕ manages
Shell (running in PTY)
```

## Build

```bash
cd gomux-term
cargo build --release
```

This creates:
- `target/release/libgomux_term.so` (or `.dylib` on macOS, `.dll` on Windows)

## Usage in Go

```go
import "gomux/pkg/gomux"

// Create terminal	erm, err := gomux.NewAlacrittyTerm(24, 80)
if err != nil {
    log.Fatal(err)
}
defer term.Close()

// Event loop
for {
    // Check if PTY has output
    ready, _ := term.PollPTY(50)
    if ready {
        n, _ := term.ProcessPTY()
        if n > 0 {
            // Re-render
            for row := 0; row < 24; row++ {
                line := term.GetLine(row)
                fmt.Print(line)
            }
        }
    }
    
    // Handle input
    term.Write([]byte("ls\r"))
}
```

## API

- `gomux_term_new(rows, cols)` - Create terminal with PTY
- `gomux_term_process_pty()` - Read PTY output, update grid
- `gomux_term_write(data, len)` - Send input to PTY
- `gomux_term_get_line(row, buf, max_len)` - Get rendered line
- `gomux_term_get_cursor(row, col)` - Get cursor position

## Limitations

Current prototype is simplified. Full implementation would need:
- Proper UTF-8 handling for wide characters
- Color/attribute extraction from cells
- Scrollback buffer access
- Better PTY polling (using mio)
