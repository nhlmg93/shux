package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"shux/internal/cfg"
	"shux/internal/protocol"
)

type copyPoint struct {
	Row int
	Col int
}

type copySelection struct {
	Anchor copyPoint
	Active bool
}

func (m Model) enterCopyMode() Model {
	m.CopyMode = true
	m.CopySelection = copySelection{}
	screen := m.paneScreen(m.ActivePaneID)
	row, col := copyCursorForScreen(screen)
	m.CopyCursor = copyPoint{Row: row, Col: col}
	return m
}

func (m Model) exitCopyMode() Model {
	m.CopyMode = false
	m.CopySelection = copySelection{}
	return m
}

func copyCursorForScreen(screen protocol.EventPaneScreenChanged) (row int, col int) {
	row = len(screen.Lines) - 1
	if row < 0 {
		row = 0
	}
	if screen.Cursor.Visible {
		row = screen.Cursor.Row
		col = screen.Cursor.Col
	}
	p := clampPoint(screen, copyPoint{Row: row, Col: col})
	return p.Row, p.Col
}

func (m Model) handleCopyModeKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := normalizeCopyKey(msg)
	binding, ok := m.Keymaps.Lookup("copy_mode", key)
	if !ok {
		return m, nil
	}
	if binding.LuaCallback != 0 {
		if m.Lua != nil {
			m.Lua.CallKeymapRef(binding.LuaCallback)
		}
		return m, nil
	}
	m, cmd, exitAfter := m.dispatchCopyBuiltin(binding.Builtin)
	if exitAfter {
		m = m.exitCopyMode()
	}
	return m, cmd
}

func normalizeCopyKey(msg tea.KeyPressMsg) string {
	key := strings.ToLower(strings.TrimSpace(msg.Key().String()))
	switch key {
	case " ":
		return "space"
	case "esc":
		return "escape"
	case "pgup":
		return "pageup"
	case "pgdown":
		return "pagedown"
	case "return":
		return "enter"
	}
	if strings.HasPrefix(key, "shift+") {
		return key
	}
	k := msg.Key()
	if k.Mod&tea.ModShift != 0 && len(key) == 1 {
		r, _ := utf8.DecodeRuneInString(key)
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return "shift+" + key
		}
	default:
		return key
	}
}

func (m Model) dispatchCopyBuiltin(action cfg.BuiltinKeyAction) (Model, tea.Cmd, bool) {
	screen := m.paneScreen(m.ActivePaneID)
	switch action {
	case cfg.ActionCopyLeft:
		m.CopyCursor.Col--
		m.clampCopyModeCursor(screen)
	case cfg.ActionCopyRight:
		m.CopyCursor.Col++
		m.clampCopyModeCursor(screen)
	case cfg.ActionCopyUp:
		if m.CopyCursor.Row > 0 {
			m.CopyCursor.Row--
			m.clampCopyModeCursor(screen)
			return m, nil, false
		}
		return m, m.dispatch(m.copyScrollCommand(-1)), false
	case cfg.ActionCopyDown:
		maxRow := max(0, len(screen.Lines)-1)
		if m.CopyCursor.Row < maxRow {
			m.CopyCursor.Row++
			m.clampCopyModeCursor(screen)
			return m, nil, false
		}
		return m, m.dispatch(m.copyScrollCommand(1)), false
	case cfg.ActionCopyWordForward:
		m.CopyCursor.Col = nextWordCol(lineText(screen, m.CopyCursor.Row), m.CopyCursor.Col)
		m.clampCopyModeCursor(screen)
	case cfg.ActionCopyWordBackward:
		m.CopyCursor.Col = prevWordCol(lineText(screen, m.CopyCursor.Row), m.CopyCursor.Col)
		m.clampCopyModeCursor(screen)
	case cfg.ActionCopyTop:
		m.CopyCursor.Row = 0
		m.CopyCursor.Col = 0
		m.clampCopyModeCursor(screen)
		return m, m.dispatch(m.copyScrollCommand(-1000)), false
	case cfg.ActionCopyBottom:
		m.CopyCursor.Row = max(0, len(screen.Lines)-1)
		m.CopyCursor.Col = 0
		m.clampCopyModeCursor(screen)
		return m, m.dispatch(m.copyScrollCommand(1000)), false
	case cfg.ActionCopyPageUp:
		delta := -max(1, m.Layout.WindowRows/2)
		return m, m.dispatch(m.copyScrollCommand(delta)), false
	case cfg.ActionCopyPageDown:
		delta := max(1, m.Layout.WindowRows/2)
		return m, m.dispatch(m.copyScrollCommand(delta)), false
	case cfg.ActionCopySelectStart:
		m.clampCopyModeCursor(screen)
		m.CopySelection = copySelection{Anchor: m.CopyCursor, Active: true}
	case cfg.ActionCopyYankSelection:
		m.clampCopyModeCursor(screen)
		m.CopyRegister = m.copySelectionText(screen)
		return m, nil, true
	case cfg.ActionCopyCancel:
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) copyModeScreenOverlay(screen protocol.EventPaneScreenChanged) protocol.EventPaneScreenChanged {
	if !m.CopyMode {
		return screen
	}
	screen.Cursor = protocol.NewEventPaneScreenCursor(m.CopyCursor.Col, m.CopyCursor.Row, false)
	start, end, ok := selectionBounds(m.CopySelection, m.CopyCursor)
	if !ok {
		return screen
	}
	start = clampPoint(screen, start)
	end = clampPoint(screen, end)
	lines := make([]protocol.EventPaneScreenLine, len(screen.Lines))
	copy(lines, screen.Lines)
	for row := start.Row; row <= end.Row && row < len(lines); row++ {
		line := lines[row]
		if len(line.Cells) == 0 {
			continue
		}
		cells := make([]protocol.EventPaneScreenCell, len(line.Cells))
		copy(cells, line.Cells)
		colStart := 0
		colEnd := len(cells) - 1
		if row == start.Row {
			colStart = min(start.Col, colEnd)
		}
		if row == end.Row {
			colEnd = min(end.Col, colEnd)
		}
		for col := colStart; col <= colEnd; col++ {
			cells[col].Inverse = !cells[col].Inverse
		}
		line.Cells = cells
		lines[row] = line
	}
	screen.Lines = lines
	return screen
}

func (m Model) copySelectionText(screen protocol.EventPaneScreenChanged) string {
	if !m.CopySelection.Active {
		return strings.TrimRight(lineText(screen, m.CopyCursor.Row), " ")
	}
	start, end := m.CopySelection.Anchor, m.CopyCursor
	if comparePoints(end, start) < 0 {
		start, end = end, start
	}
	start = clampPoint(screen, start)
	end = clampPoint(screen, end)
	var b strings.Builder
	for row := start.Row; row <= end.Row; row++ {
		line := []rune(lineText(screen, row))
		if len(line) == 0 {
			if row != end.Row {
				b.WriteByte('\n')
			}
			continue
		}
		colStart := 0
		colEnd := len(line) - 1
		if row == start.Row {
			colStart = min(start.Col, colEnd)
		}
		if row == end.Row {
			colEnd = min(end.Col, colEnd)
		}
		if colEnd >= colStart {
			b.WriteString(string(line[colStart : colEnd+1]))
		}
		if row != end.Row {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (m Model) copyScrollCommand(delta int) protocol.CommandPaneScroll {
	return protocol.CommandPaneScroll{
		SessionID: m.SessionID,
		WindowID:  m.WindowID,
		PaneID:    m.ActivePaneID,
		Delta:     delta,
	}
}

func (m *Model) clampCopyModeCursor(screen protocol.EventPaneScreenChanged) {
	m.CopyCursor = clampPoint(screen, m.CopyCursor)
}

func clampPoint(screen protocol.EventPaneScreenChanged, p copyPoint) copyPoint {
	maxRow := max(0, len(screen.Lines)-1)
	p.Row = min(max(0, p.Row), maxRow)
	maxCol := max(0, utf8.RuneCountInString(lineText(screen, p.Row))-1)
	p.Col = min(max(0, p.Col), maxCol)
	return p
}

func lineText(screen protocol.EventPaneScreenChanged, row int) string {
	if row < 0 || row >= len(screen.Lines) {
		return ""
	}
	return screen.Lines[row].Text
}

func nextWordCol(line string, col int) int {
	runes := []rune(line)
	if len(runes) == 0 {
		return 0
	}
	i := min(max(col+1, 0), len(runes)-1)
	for i < len(runes) && isWordRune(runes[i]) {
		i++
	}
	for i < len(runes) && !isWordRune(runes[i]) {
		i++
	}
	if i >= len(runes) {
		return len(runes) - 1
	}
	return i
}

func prevWordCol(line string, col int) int {
	runes := []rune(line)
	if len(runes) == 0 {
		return 0
	}
	i := min(max(col-1, 0), len(runes)-1)
	for i > 0 && !isWordRune(runes[i]) {
		i--
	}
	for i > 0 && isWordRune(runes[i-1]) {
		i--
	}
	return i
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func comparePoints(a, b copyPoint) int {
	if a.Row < b.Row {
		return -1
	}
	if a.Row > b.Row {
		return 1
	}
	if a.Col < b.Col {
		return -1
	}
	if a.Col > b.Col {
		return 1
	}
	return 0
}

func selectionBounds(sel copySelection, cursor copyPoint) (copyPoint, copyPoint, bool) {
	if !sel.Active {
		return copyPoint{}, copyPoint{}, false
	}
	start, end := sel.Anchor, cursor
	if comparePoints(end, start) < 0 {
		start, end = end, start
	}
	return start, end, true
}
