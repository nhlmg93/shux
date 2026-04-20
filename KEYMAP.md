# shux Key Mapping

## Prefix Key: Ctrl+B

When you press `Ctrl+B`, shux enters prefix mode. Next key is interpreted as a shux command.

## shux Commands (Prefix Mode)

| Key | Command |
|-----|---------|
| `Ctrl+B` `w` | Create new window |
| `Ctrl+B` `n` | Next window |
| `Ctrl+B` `p` | Previous window |
| `Ctrl+B` `q` | Quit shux |
| `Ctrl+B` `c` | Create new pane (todo) |
| `Ctrl+B` `d` | Detach (todo: snapshot & exit) |
| `Ctrl+B` number | Switch to window N (todo) |

## Shell/Readline Keys (Pass Through)

These keys are sent directly to the shell without interception:

### Navigation
| Key | Code | Function |
|-----|------|----------|
| `Ctrl+A` | 0x01 | Beginning of line |
| `Ctrl+E` | 0x05 | End of line |
| `Ctrl+F` | 0x06 | Forward one char |
| `Ctrl+B` | 0x02 | Backward one char (except when prefix) |
| `Alt+F` | ESC f | Forward one word |
| `Alt+B` | ESC b | Backward one word |
| `Ctrl+P` | 0x10 | Previous history (up) |
| `Ctrl+N` | 0x0E | Next history (down) |

### Editing
| Key | Code | Function |
|-----|------|----------|
| `Ctrl+D` | 0x04 | Delete char / EOF |
| `Ctrl+H` | 0x08 | Backspace (same as Backspace key) |
| `Ctrl+W` | 0x17 | Delete word backward |
| `Alt+D` | ESC d | Delete word forward |
| `Ctrl+K` | 0x0B | Kill to end of line |
| `Ctrl+U` | 0x15 | Kill to beginning of line |
| `Ctrl+Y` | 0x19 | Yank (paste) |
| `Ctrl+T` | 0x14 | Transpose chars |
| `Ctrl+L` | 0x0C | Clear screen |
| `Tab` | 0x09 | Auto-complete |
| `Ctrl+R` | 0x12 | Reverse history search |
| `Ctrl+G` | 0x07 | Cancel search |

### Arrows
| Key | Escape Sequence | Function |
|-----|-----------------|----------|
| `↑` | ESC [ A | Previous history |
| `↓` | ESC [ B | Next history |
| `→` | ESC [ C | Forward char |
| `←` | ESC [ D | Backward char |
| `Home` | ESC [ H / ESC [ 1 ~ | Beginning of line |
| `End` | ESC [ F / ESC [ 4 ~ | End of line |
| `PgUp` | ESC [ 5 ~ | Scroll up |
| `PgDn` | ESC [ 6 ~ | Scroll down |
| `Delete` | ESC [ 3 ~ | Delete char |
| `Insert` | ESC [ 2 ~ | Toggle insert mode |

### Function Keys
| Key | Escape Sequence |
|-----|-----------------|
| `F1` | ESC [ P / ESC OP |
| `F2` | ESC [ Q / ESC OQ |
| `F3` | ESC [ R / ESC OR |
| `F4` | ESC [ S / ESC OS |
| `F5` | ESC [ 15 ~ |
| `F6` | ESC [ 17 ~ |
| `F7` | ESC [ 18 ~ |
| `F8` | ESC [ 19 ~ |
| `F9` | ESC [ 20 ~ |
| `F10` | ESC [ 21 ~ |
| `F11` | ESC [ 23 ~ |
| `F12` | ESC [ 24 ~ |

### Special
| Key | Code | Function |
|-----|------|----------|
| `Enter` | \r (0x0D) | Execute command |
| `Return` | \r | Same as Enter |
| `Backspace` | 0x7F / 0x08 | Delete backward |
| `Escape` | 0x1B | Cancel / Meta prefix |
| `Ctrl+J` | 0x0A | Same as Enter (newline) |
| `Ctrl+M` | 0x0D | Same as Enter (carriage return) |
| `Ctrl+I` | 0x09 | Same as Tab |
| `Ctrl+[` | 0x1B | Same as Escape |

### Ctrl + Letter (ASCII codes)
| Key | Code | Hex |
|-----|------|-----|
| `Ctrl+@` | 0 | 0x00 |
| `Ctrl+A` | 1 | 0x01 |
| `Ctrl+B` | 2 | 0x02 |
| `Ctrl+C` | 3 | 0x03 |
| `Ctrl+D` | 4 | 0x04 |
| `Ctrl+E` | 5 | 0x05 |
| `Ctrl+F` | 6 | 0x06 |
| `Ctrl+G` | 7 | 0x07 |
| `Ctrl+H` | 8 | 0x08 |
| `Ctrl+I` | 9 | 0x09 (Tab) |
| `Ctrl+J` | 10 | 0x0A (LF) |
| `Ctrl+K` | 11 | 0x0B |
| `Ctrl+L` | 12 | 0x0C |
| `Ctrl+M` | 13 | 0x0D (CR) |
| `Ctrl+N` | 14 | 0x0E |
| `Ctrl+O` | 15 | 0x0F |
| `Ctrl+P` | 16 | 0x10 |
| `Ctrl+Q` | 17 | 0x11 |
| `Ctrl+R` | 18 | 0x12 |
| `Ctrl+S` | 19 | 0x13 |
| `Ctrl+T` | 20 | 0x14 |
| `Ctrl+U` | 21 | 0x15 |
| `Ctrl+V` | 22 | 0x16 |
| `Ctrl+W` | 23 | 0x17 |
| `Ctrl+X` | 24 | 0x18 |
| `Ctrl+Y` | 25 | 0x19 |
| `Ctrl+Z` | 26 | 0x1A |
| `Ctrl+[` | 27 | 0x1B (Escape) |
| `Ctrl+\` | 28 | 0x1C |
| `Ctrl+]` | 29 | 0x1D |
| `Ctrl+^` | 30 | 0x1E |
| `Ctrl+_` | 31 | 0x1F |

## Implementation Notes

### What Bubble Tea Gives Us

- `key.Type`: tea.KeyEnter, tea.KeyBackspace, tea.KeyCtrlC, etc.
- `key.Runes`: Unicode runes for printable characters
- `key.Alt`: Boolean indicating Alt was pressed

### What We Need to Handle

1. **Ctrl+Letter**: Send byte 1-26 (except Ctrl+B which is prefix)
2. **Alt+Key**: Send ESC (0x1B) followed by the key
3. **Special keys**: Map to correct escape sequences
4. **Arrow keys**: Bubble Tea handles, send CSI sequences

### Missing in Current Implementation

- [ ] Ctrl+A, E, F, K, U, Y, etc. (most Ctrl keys)
- [ ] Alt combinations (Alt+F, Alt+B, etc.)
- [ ] Function keys F1-F12
- [ ] Home, End, PgUp, PgDn, Insert, Delete
- [ ] Proper escape sequence generation
