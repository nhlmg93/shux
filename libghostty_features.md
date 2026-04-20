# libghostty Features We Could Leverage Better

## Currently Using
✅ Basic terminal emulation (VTWrite, GridRef)  
✅ Cell text content (codepoint)  
✅ Cursor position (RenderState)  
✅ Resize  
✅ Scrollback (via WithMaxScrollback)

## Underutilized Features

### 1. Terminal Effects (Callbacks)
**What:** React to terminal events
```go
libghostty.WithTitleChanged(func(t *libghostty.Terminal) {
    // Window title changed via OSC 0/2
    // Could update gomux window name
})

libghostty.WithBell(func(t *libghostty.Terminal) {
    // Bell character received
    // Could flash screen or play sound
})

libghostty.WithWritePty(func(t *libghostty.Terminal, data []byte) {
    // Terminal wants to write back to PTY
    // Used for query responses
})
```

**Use in gomux:**
- Auto-update window titles from shell
- Visual bell support
- Handle terminal queries properly

---

### 2. Cell Styling (Colors & Attributes)
**What:** Get color and style info for each cell
```go
ref, _ := term.GridRef(point)
style, _ := ref.Style()

fg := style.FgColor()  // RGB color
bg := style.BgColor()  
bold := style.Bold()
italic := style.Italic()
underline := style.Underline()
```

**Use in gomux:**
- Currently we render plain text only
- Could pass color info to Bubble Tea
- Would need lipgloss integration for true color rendering

---

### 3. Hyperlinks (OSC 8)
**What:** Detect clickable terminal hyperlinks
```go
hasLink, _ := cell.HasHyperlink()
if hasLink {
    // Get hyperlink URL
}
```

**Use in gomux:**
- Make URLs clickable in UI
- Would need mouse support + hit testing

---

### 4. Kitty Graphics Protocol
**What:** Display images in terminal
```go
kg, _ := term.KittyGraphics()
// Iterate images and placements
```

**Use in gomux:**
- Mostly for completeness
- Would need image rendering in Bubble Tea (complex)

---

### 5. Mouse Support
**What:** Handle mouse events from terminal
```go
// In terminal options:
libghostty.WithMouse(...)

// Mouse events:
type MouseEvent struct {
    X, Y int
    Button MouseButton
    Action MouseAction // press, release, drag
}
```

**Use in gomux:**
- Click to focus pane
- Click URLs
- Scroll with mouse wheel

---

### 6. RenderState Optimization
**What:** Reuse RenderState instead of creating each time
```go
// Current (creates/destroys each frame):
rs, _ := libghostty.NewRenderState()
defer rs.Close()
rs.Update(term)

// Better (reuse):
type Term struct {
    renderState *libghostty.RenderState
}

func (t *Term) GetContent() {
    t.renderState.Update(t.term)  // Reuse existing
    cursorX, _ := t.renderState.CursorViewportX()
    // ...
}
```

**Performance:** Small win, but cleaner

---

### 7. Better Cell Data Access
**What:** Direct cell data queries
```go
// Instead of:
cell, _ := ref.Cell()
hasText, _ := cell.HasText()
cp, _ := cell.Codepoint()

// Could use:
cellData, _ := screen.GetCellData(row, col, libghostty.CellDataCodepoint)
```

---

### 8. Mode Queries
**What:** Check terminal state
```go
// Is alt screen active?
altScreen, _ := term.ModeGet(libghostty.ModeAltScreen)

// Is cursor visible?
cursorVisible, _ := term.ModeGet(libghostty.ModeCursorVisible)

// Application keypad mode?
// etc.
```

**Use in gomux:**
- Know when vim/less is running (alt screen)
- Adjust UI behavior

---

## Priority Recommendations

### High Impact, Low Effort:
1. **Reuse RenderState** - One line change, slightly cleaner
2. **Terminal Effects** - Add WithTitleChanged, WithBell for polish

### High Impact, Medium Effort:
3. **Cell Styling** - Would need UI changes to display colors
4. **Mouse Support** - Would need input handling changes

### Low Priority:
5. **Kitty Graphics** - Complex, niche use case
6. **Hyperlinks** - Requires mouse support first

## Quick Wins to Implement

```go
// In New(), add effect handlers:
ghosttyTerm, err := libghostty.NewTerminal(
    libghostty.WithSize(uint16(cols), uint16(rows)),
    libghostty.WithMaxScrollback(10000),
    libghostty.WithTitleChanged(func(t *libghostty.Terminal) {
        // Could emit message to update window title
    }),
    libghostty.WithBell(func(t *libghostty.Terminal) {
        // Could flash screen
    }),
)

// In Term struct, cache RenderState:
type Term struct {
    // ... existing fields ...
    renderState *libghostty.RenderState  // Cache this
}

// In handleAsk, reuse:
if t.renderState == nil {
    t.renderState, _ = libghostty.NewRenderState()
}
t.renderState.Update(t.term)
// Use t.renderState.CursorViewportX/Y
```

## Next Steps

**Option A: Minimal** - Just add title changes and bell (2 lines)  
**Option B: Colors** - Add style extraction, update UI rendering (more work)  
**Option C: Mouse** - Add mouse support for pane clicking (architectural change)

Which direction interests you?
