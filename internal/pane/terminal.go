package pane

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"github.com/creack/pty"
	"github.com/mitchellh/go-libghostty"
	"shux/internal/actor"
	"shux/internal/protocol"
)

// Default cell pixel size for libghostty Resize, matching go-libghostty test usage.
// Init uses NewTerminal(WithSize); Resize must be given explicit cell dimensions.
const defaultCellW, defaultCellH = 8, 16

type ptyOutput []byte

// Terminal owns the running terminal state for a pane: shell process, PTY,
// libghostty VT, render state, dimensions, and screen revision.
type Terminal struct {
	ShellPath string

	VT           *libghostty.Terminal
	RenderState  *libghostty.RenderState
	KeyEncoder   *libghostty.KeyEncoder
	MouseEncoder *libghostty.MouseEncoder
	PTY          *os.File
	Shell        *exec.Cmd

	cols     uint16
	rows     uint16
	revision uint64
}

func NewTerminal(shellPath string) *Terminal {
	if shellPath == "" {
		panic("pane: NewTerminal: empty shell path")
	}
	return &Terminal{ShellPath: shellPath}
}

// Init allocates libghostty and starts the shell PTY exactly once.
func (t *Terminal) Init(ctx context.Context, self actor.Ref[protocol.Command], cols, rows uint16) protocol.EventPaneScreenChanged {
	if cols == 0 || rows == 0 {
		panic(fmt.Sprintf("pane: terminal init: invalid size %dx%d (cols and rows must be positive)", cols, rows))
	}
	if t.VT != nil || t.PTY != nil || t.Shell != nil {
		panic("pane: terminal init: already created (double init)")
	}
	term, err := libghostty.NewTerminal(libghostty.WithSize(cols, rows))
	if err != nil {
		panic(fmt.Sprintf("pane: NewTerminal: %v", err))
	}
	renderState, err := libghostty.NewRenderState()
	if err != nil {
		term.Close()
		panic(fmt.Sprintf("pane: NewRenderState: %v", err))
	}
	keyEncoder, err := libghostty.NewKeyEncoder()
	if err != nil {
		renderState.Close()
		term.Close()
		panic(fmt.Sprintf("pane: NewKeyEncoder: %v", err))
	}
	mouseEncoder, err := libghostty.NewMouseEncoder()
	if err != nil {
		keyEncoder.Close()
		renderState.Close()
		term.Close()
		panic(fmt.Sprintf("pane: NewMouseEncoder: %v", err))
	}
	cmd := exec.Command(t.ShellPath)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
	if err != nil {
		mouseEncoder.Close()
		keyEncoder.Close()
		renderState.Close()
		term.Close()
		panic(fmt.Sprintf("pane: pty start %q: %v", t.ShellPath, err))
	}
	t.VT = term
	t.RenderState = renderState
	t.KeyEncoder = keyEncoder
	t.MouseEncoder = mouseEncoder
	t.PTY = ptyFile
	t.Shell = cmd
	t.cols = cols
	t.rows = rows
	t.revision++
	go readPTY(ctx, self, ptyFile)
	return t.ScreenChanged()
}

func readPTY(ctx context.Context, self actor.Ref[protocol.Command], ptyFile *os.File) {
	buf := make([]byte, 4096)
	for {
		n, err := ptyFile.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			if sendErr := self.Send(ctx, ptyOutput(chunk)); sendErr != nil {
				return
			}
		}
		if err != nil {
			if err == io.EOF {
				return
			}
			return
		}
	}
}

func (t *Terminal) FeedOutput(chunk []byte) (protocol.EventPaneScreenChanged, bool) {
	if t.VT == nil {
		panic("pane: terminal output before init")
	}
	if len(chunk) == 0 {
		return protocol.EventPaneScreenChanged{}, false
	}
	t.VT.VTWrite(chunk)
	t.revision++
	return t.ScreenChanged(), true
}

func (t *Terminal) Resize(cols, rows uint16) protocol.EventPaneScreenChanged {
	if cols == 0 || rows == 0 {
		panic(fmt.Sprintf("pane: terminal resize: invalid size %dx%d (cols and rows must be positive)", cols, rows))
	}
	if t.VT == nil {
		panic("pane: terminal resize: terminal not created (resize before init)")
	}
	if t.PTY == nil {
		panic("pane: terminal resize: pty not created (resize before init)")
	}
	if err := pty.Setsize(t.PTY, &pty.Winsize{Cols: cols, Rows: rows}); err != nil {
		panic(fmt.Sprintf("pane: pty resize: %v", err))
	}
	if err := t.VT.Resize(cols, rows, defaultCellW, defaultCellH); err != nil {
		panic(fmt.Sprintf("pane: Resize: %v", err))
	}
	t.cols = cols
	t.rows = rows
	t.revision++
	return t.ScreenChanged()
}

func (t *Terminal) ScreenChanged() protocol.EventPaneScreenChanged {
	if t.RenderState == nil {
		panic("pane: terminal screen before render state")
	}
	if err := t.RenderState.Update(t.VT); err != nil {
		panic(fmt.Sprintf("pane: RenderState.Update: %v", err))
	}
	return protocol.EventPaneScreenChanged{
		Revision: t.revision,
		Cols:     int(t.cols),
		Rows:     int(t.rows),
		Lines:    t.screenLines(),
		Cursor:   t.cursorState(),
	}
}

func (t *Terminal) cursorState() protocol.EventPaneScreenCursor {
	visible, err := t.RenderState.CursorVisible()
	if err != nil {
		panic(fmt.Sprintf("pane: RenderState.CursorVisible: %v", err))
	}
	blink, err := t.RenderState.CursorBlinking()
	if err != nil {
		panic(fmt.Sprintf("pane: RenderState.CursorBlinking: %v", err))
	}
	hasValue, err := t.RenderState.CursorViewportHasValue()
	if err != nil {
		panic(fmt.Sprintf("pane: RenderState.CursorViewportHasValue: %v", err))
	}
	if !visible || !hasValue {
		return protocol.EventPaneScreenCursor{Blink: blink}
	}
	x, err := t.RenderState.CursorViewportX()
	if err != nil {
		panic(fmt.Sprintf("pane: RenderState.CursorViewportX: %v", err))
	}
	y, err := t.RenderState.CursorViewportY()
	if err != nil {
		panic(fmt.Sprintf("pane: RenderState.CursorViewportY: %v", err))
	}
	if x >= t.cols || y >= t.rows {
		return protocol.EventPaneScreenCursor{Blink: blink}
	}
	return protocol.NewEventPaneScreenCursor(int(x), int(y), blink)
}

func (t *Terminal) HandleKey(cmd protocol.CommandPaneKey) {
	if t.VT == nil || t.KeyEncoder == nil || t.PTY == nil {
		panic("pane: key input before init")
	}
	event, err := libghostty.NewKeyEvent()
	if err != nil {
		panic(fmt.Sprintf("pane: NewKeyEvent: %v", err))
	}
	defer event.Close()
	event.SetAction(keyAction(cmd.Action))
	event.SetMods(inputMods(cmd.Modifiers))
	text := cmd.Text
	if text == "" && utf8.RuneCountInString(cmd.Key) == 1 {
		text = cmd.Key
	}
	event.SetUTF8(text)
	event.SetKey(keyCode(cmd.Key))
	t.KeyEncoder.SetOptFromTerminal(t.VT)
	encoded, err := t.KeyEncoder.Encode(event)
	if err != nil {
		panic(fmt.Sprintf("pane: key encode: %v", err))
	}
	t.writePTY(encoded)
}

func (t *Terminal) HandleMouse(cmd protocol.CommandPaneMouse) {
	if t.VT == nil || t.MouseEncoder == nil || t.PTY == nil {
		panic("pane: mouse input before init")
	}
	event, err := libghostty.NewMouseEvent()
	if err != nil {
		panic(fmt.Sprintf("pane: NewMouseEvent: %v", err))
	}
	defer event.Close()
	event.SetAction(mouseAction(cmd.Action))
	if button, ok := mouseButton(cmd.Button); ok {
		event.SetButton(button)
	} else {
		event.ClearButton()
	}
	event.SetMods(inputMods(cmd.Modifiers))
	event.SetPosition(libghostty.MousePosition{
		X: float32(cmd.CellCol*defaultCellW + defaultCellW/2),
		Y: float32(cmd.CellRow*defaultCellH + defaultCellH/2),
	})
	t.MouseEncoder.SetOptFromTerminal(t.VT)
	t.MouseEncoder.SetOptSize(libghostty.MouseEncoderSize{
		ScreenWidth:  uint32(t.cols) * defaultCellW,
		ScreenHeight: uint32(t.rows) * defaultCellH,
		CellWidth:    defaultCellW,
		CellHeight:   defaultCellH,
	})
	encoded, err := t.MouseEncoder.Encode(event)
	if err != nil {
		panic(fmt.Sprintf("pane: mouse encode: %v", err))
	}
	t.writePTY(encoded)
}

func (t *Terminal) HandlePaste(cmd protocol.CommandPanePaste) {
	if t.PTY == nil {
		panic("pane: paste input before init")
	}
	t.writePTY(cmd.Data)
}

func (t *Terminal) writePTY(data []byte) {
	if len(data) == 0 {
		return
	}
	n, err := t.PTY.Write(data)
	if err != nil {
		panic(fmt.Sprintf("pane: pty write: %v", err))
	}
	if n != len(data) {
		panic(fmt.Sprintf("pane: short pty write %d != %d", n, len(data)))
	}
}

func keyAction(action protocol.KeyAction) libghostty.KeyAction {
	switch action {
	case protocol.KeyActionRelease:
		return libghostty.KeyActionRelease
	case protocol.KeyActionRepeat:
		return libghostty.KeyActionRepeat
	default:
		return libghostty.KeyActionPress
	}
}

func mouseAction(action protocol.MouseAction) libghostty.MouseAction {
	switch action {
	case protocol.MouseActionRelease:
		return libghostty.MouseActionRelease
	case protocol.MouseActionMotion:
		return libghostty.MouseActionMotion
	default:
		return libghostty.MouseActionPress
	}
}

func mouseButton(button protocol.MouseButton) (libghostty.MouseButton, bool) {
	switch button {
	case protocol.MouseButtonLeft:
		return libghostty.MouseButtonLeft, true
	case protocol.MouseButtonMiddle:
		return libghostty.MouseButtonMiddle, true
	case protocol.MouseButtonRight:
		return libghostty.MouseButtonRight, true
	case protocol.MouseButtonWheelUp:
		return libghostty.MouseButtonFour, true
	case protocol.MouseButtonWheelDown:
		return libghostty.MouseButtonFive, true
	case protocol.MouseButtonWheelLeft:
		return libghostty.MouseButtonSix, true
	case protocol.MouseButtonWheelRight:
		return libghostty.MouseButtonSeven, true
	default:
		return libghostty.MouseButtonUnknown, false
	}
}

func inputMods(mods protocol.InputModifiers) libghostty.Mods {
	var out libghostty.Mods
	if mods&protocol.ModifierShift != 0 {
		out |= libghostty.ModShift
	}
	if mods&protocol.ModifierCtrl != 0 {
		out |= libghostty.ModCtrl
	}
	if mods&protocol.ModifierAlt != 0 {
		out |= libghostty.ModAlt
	}
	if mods&protocol.ModifierMeta != 0 {
		out |= libghostty.ModSuper
	}
	return out
}

func keyCode(key string) libghostty.Key {
	switch strings.ToLower(key) {
	case "enter":
		return libghostty.KeyEnter
	case "tab":
		return libghostty.KeyTab
	case "backspace", "ctrl+h":
		return libghostty.KeyBackspace
	case "esc", "escape":
		return libghostty.KeyEscape
	case "up":
		return libghostty.KeyArrowUp
	case "down":
		return libghostty.KeyArrowDown
	case "left":
		return libghostty.KeyArrowLeft
	case "right":
		return libghostty.KeyArrowRight
	case "home":
		return libghostty.KeyHome
	case "end":
		return libghostty.KeyEnd
	case "pgup", "pageup":
		return libghostty.KeyPageUp
	case "pgdown", "pagedown":
		return libghostty.KeyPageDown
	case "insert":
		return libghostty.KeyInsert
	case "delete":
		return libghostty.KeyDelete
	case "space", " ":
		return libghostty.KeySpace
	}
	if len(key) == 2 && (key[0] == 'f' || key[0] == 'F') && key[1] >= '1' && key[1] <= '9' {
		return libghostty.KeyF1 + libghostty.Key(key[1]-'1')
	}
	if len(key) == 3 && (key[0] == 'f' || key[0] == 'F') && key[1] == '1' && key[2] >= '0' && key[2] <= '2' {
		return libghostty.KeyF10 + libghostty.Key(key[2]-'0')
	}
	if r, n := utf8.DecodeRuneInString(key); r != utf8.RuneError && n == len(key) {
		if r >= 'a' && r <= 'z' {
			return libghostty.KeyA + libghostty.Key(r-'a')
		}
		if r >= 'A' && r <= 'Z' {
			return libghostty.KeyA + libghostty.Key(r-'A')
		}
		if r >= '0' && r <= '9' {
			return libghostty.KeyDigit0 + libghostty.Key(r-'0')
		}
	}
	return libghostty.KeyUnidentified
}

func (t *Terminal) Close() {
	if t.PTY != nil {
		_ = t.PTY.Close()
		t.PTY = nil
	}
	if t.Shell != nil && t.Shell.Process != nil {
		_ = t.Shell.Process.Kill()
		_ = t.Shell.Wait()
		t.Shell = nil
	}
	if t.MouseEncoder != nil {
		t.MouseEncoder.Close()
		t.MouseEncoder = nil
	}
	if t.KeyEncoder != nil {
		t.KeyEncoder.Close()
		t.KeyEncoder = nil
	}
	if t.RenderState != nil {
		t.RenderState.Close()
		t.RenderState = nil
	}
	if t.VT != nil {
		t.VT.Close()
		t.VT = nil
	}
}

func (t *Terminal) screenLines() []protocol.EventPaneScreenLine {
	if t.RenderState == nil {
		panic("pane: terminal screen lines: nil render state")
	}
	rowIter, err := libghostty.NewRenderStateRowIterator()
	if err != nil {
		panic(fmt.Sprintf("pane: NewRenderStateRowIterator: %v", err))
	}
	defer rowIter.Close()
	cells, err := libghostty.NewRenderStateRowCells()
	if err != nil {
		panic(fmt.Sprintf("pane: NewRenderStateRowCells: %v", err))
	}
	defer cells.Close()
	if err := t.RenderState.RowIterator(rowIter); err != nil {
		panic(fmt.Sprintf("pane: RenderState.RowIterator: %v", err))
	}
	lines := make([]protocol.EventPaneScreenLine, 0, int(t.rows))
	for row := 0; row < int(t.rows) && rowIter.Next(); row++ {
		if err := rowIter.Cells(cells); err != nil {
			panic(fmt.Sprintf("pane: RenderStateRowIterator.Cells: %v", err))
		}
		line := protocol.EventPaneScreenLine{Cells: make([]protocol.EventPaneScreenCell, 0, int(t.cols))}
		var text strings.Builder
		for col := 0; col < int(t.cols) && cells.Next(); col++ {
			cell := screenCell(cells)
			line.Cells = append(line.Cells, cell)
			text.WriteString(cell.Text)
		}
		line.Text = text.String()
		lines = append(lines, line)
	}
	return lines
}

func screenCell(cells *libghostty.RenderStateRowCells) protocol.EventPaneScreenCell {
	graphemes, err := cells.Graphemes()
	if err != nil {
		panic(fmt.Sprintf("pane: RenderStateRowCells.Graphemes: %v", err))
	}
	style, err := cells.Style()
	if err != nil {
		panic(fmt.Sprintf("pane: RenderStateRowCells.Style: %v", err))
	}
	return protocol.EventPaneScreenCell{
		Text:          graphemeString(graphemes),
		Foreground:    screenColor(style.FgColor()),
		Background:    screenColor(style.BgColor()),
		Bold:          style.Bold(),
		Italic:        style.Italic(),
		Faint:         style.Faint(),
		Blink:         style.Blink(),
		Inverse:       style.Inverse(),
		Invisible:     style.Invisible(),
		Underline:     style.Underline() != libghostty.UnderlineNone,
		Strikethrough: style.Strikethrough(),
		Overline:      style.Overline(),
	}
}

func graphemeString(graphemes []uint32) string {
	if len(graphemes) == 0 {
		return " "
	}
	runes := make([]rune, 0, len(graphemes))
	for _, g := range graphemes {
		runes = append(runes, rune(g))
	}
	return string(runes)
}

func screenColor(color libghostty.StyleColor) protocol.EventPaneScreenColor {
	switch color.Tag {
	case libghostty.StyleColorPalette:
		return protocol.EventPaneScreenColor{Kind: "palette", Index: color.Palette}
	case libghostty.StyleColorRGB:
		return protocol.EventPaneScreenColor{Kind: "rgb", R: color.RGB.R, G: color.RGB.G, B: color.RGB.B}
	default:
		return protocol.EventPaneScreenColor{Kind: "default"}
	}
}
