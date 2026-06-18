package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"shux/internal/protocol"
)

const treeModeHighlightANSI = "\x1b[48;5;220m\x1b[38;5;16m"
const treeModeMarkedANSI = "\x1b[7m"
const treeModeHelp = "↑↓ nav  ←→ expand  Enter select  t tag  f filter  / search  O sort  J/K swap  x kill  Esc cancel"

func (m Model) handleTreeViewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.TreeView.PromptKind != treePromptNone {
		return m.handleTreePromptKey(msg)
	}
	key := msg.Key().String()
	if sel, ok := m.treeShortcutSelect(key); ok {
		m.TreeView.Cursor = sel
		return m.treeSelectCurrent()
	}
	switch key {
	case "esc", "ctrl+[", "ctrl+g", "q":
		return m.exitTreeView(), nil
	case "up", "k", "ctrl+p":
		if m.TreeView.Cursor > 0 {
			m.TreeView.Cursor--
		}
		return m, nil
	case "down", "j", "ctrl+n":
		if m.TreeView.Cursor < len(m.TreeView.Items)-1 {
			m.TreeView.Cursor++
		}
		return m, nil
	case "g", "home":
		m.TreeView.Cursor = 0
		return m, nil
	case "G", "end":
		if len(m.TreeView.Items) > 0 {
			m.TreeView.Cursor = len(m.TreeView.Items) - 1
		}
		return m, nil
	case "left", "h", "-":
		return m.treeCollapseCurrent(), nil
	case "right", "l", "+":
		return m.treeExpandCurrent(), nil
	case "H":
		return m.treeJumpToHere(), nil
	case "enter":
		return m.treeSelectCurrent()
	case "x":
		return m.treeKillCurrent()
	case "X":
		return m.treeKillTagged()
	case "t":
		return m.treeToggleTagCurrent(), nil
	case "T":
		m.TreeView.Tagged = make(map[treeItemID]bool)
		m.rebuildTreeItems()
		return m, nil
	case "f":
		m.TreeView.PromptKind = treePromptFilter
		m.TreeView.PromptInput = m.TreeView.Filter
		return m, nil
	case "/", "?":
		m.TreeView.PromptKind = treePromptSearch
		m.TreeView.PromptInput = m.TreeView.SearchQuery
		m.TreeView.SearchForward = true
		return m, nil
	case "n":
		m.TreeView.SearchForward = true
		m.TreeView.Cursor = m.treeSearchFrom(m.TreeView.Cursor, true)
		return m, nil
	case "N":
		m.TreeView.SearchForward = false
		m.TreeView.Cursor = m.treeSearchFrom(m.TreeView.Cursor, false)
		return m, nil
	case "O":
		switch m.TreeView.SortOrder {
		case treeSortIndex:
			m.TreeView.SortOrder = treeSortName
		default:
			m.TreeView.SortOrder = treeSortIndex
		}
		m.rebuildTreeItems()
		return m, nil
	case "r":
		m.TreeView.SortReversed = !m.TreeView.SortReversed
		m.rebuildTreeItems()
		return m, nil
	case "J":
		return m.treeSwapWindow(1)
	case "K":
		return m.treeSwapWindow(-1)
	case "m":
		return m.treeMarkCurrent(), nil
	case "M":
		m.TreeView.Marked = treeItemID{}
		return m, nil
	case "<":
		m.TreeView.PreviewOffset--
		return m, nil
	case ">":
		m.TreeView.PreviewOffset++
		return m, nil
	}
	return m, nil
}

func (m Model) treeShortcutSelect(key string) (int, bool) {
	for i, it := range m.TreeView.Items {
		if it.shortcutKey == key {
			return i, true
		}
	}
	return 0, false
}

func (m Model) handleTreePromptKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key().String()
	switch key {
	case "esc", "ctrl+[", "ctrl+g":
		m.TreeView.PromptKind = treePromptNone
		m.TreeView.PromptInput = ""
		return m, nil
	case "enter":
		input := strings.TrimSpace(m.TreeView.PromptInput)
		switch m.TreeView.PromptKind {
		case treePromptFilter:
			m.TreeView.Filter = input
			m.rebuildTreeItems()
		case treePromptSearch:
			m.TreeView.SearchQuery = input
			if input != "" {
				m.TreeView.Cursor = m.treeSearchFrom(m.TreeView.Cursor, m.TreeView.SearchForward)
			}
		}
		m.TreeView.PromptKind = treePromptNone
		m.TreeView.PromptInput = ""
		return m, nil
	case "backspace", "ctrl+h":
		if m.TreeView.PromptInput != "" {
			m.TreeView.PromptInput = m.TreeView.PromptInput[:len(m.TreeView.PromptInput)-1]
		}
		return m, nil
	default:
		if len(key) == 1 && key[0] >= 32 && key[0] != 127 {
			m.TreeView.PromptInput += key
		}
		return m, nil
	}
}

func (m Model) treeToggleTagCurrent() Model {
	if m.TreeView.Cursor < 0 || m.TreeView.Cursor >= len(m.TreeView.Items) {
		return m
	}
	it := m.TreeView.Items[m.TreeView.Cursor]
	if m.TreeView.Tagged == nil {
		m.TreeView.Tagged = make(map[treeItemID]bool)
	}
	if m.TreeView.Tagged[it.id] {
		delete(m.TreeView.Tagged, it.id)
	} else {
		m.treeUntagDescendants(it.id)
		m.treeUntagAncestors(it.parent)
		m.TreeView.Tagged[it.id] = true
	}
	m.rebuildTreeItems()
	m.TreeView.Cursor = m.treeIndexOf(it.sessionID, it.windowID, it.paneID, it.kind)
	if m.TreeView.Cursor < len(m.TreeView.Items)-1 {
		m.TreeView.Cursor++
	}
	return m
}

func (m *Model) treeUntagAncestors(parent int) {
	for parent >= 0 && parent < len(m.TreeView.Items) {
		delete(m.TreeView.Tagged, m.TreeView.Items[parent].id)
		parent = m.TreeView.Items[parent].parent
	}
}

func (m *Model) treeUntagDescendants(id treeItemID) {
	for _, tagged := range m.TreeView.Items {
		if m.treeItemIsDescendant(tagged, id) {
			delete(m.TreeView.Tagged, tagged.id)
		}
	}
}

func (m Model) treeItemIsDescendant(it treeItem, ancestor treeItemID) bool {
	parent := it.parent
	for parent >= 0 && parent < len(m.TreeView.Items) {
		if m.TreeView.Items[parent].id == ancestor {
			return true
		}
		parent = m.TreeView.Items[parent].parent
	}
	return false
}

func (m Model) treeMarkCurrent() Model {
	if m.TreeView.Cursor < 0 || m.TreeView.Cursor >= len(m.TreeView.Items) {
		return m
	}
	it := m.TreeView.Items[m.TreeView.Cursor]
	if it.kind == treeItemPane {
		m.TreeView.Marked = it.id
	}
	return m
}

func (m Model) treeSwapWindow(direction int) (Model, tea.Cmd) {
	w1, w2, ok := m.treeAdjacentWindow(m.TreeView.Cursor, direction)
	if !ok {
		return m, nil
	}
	it := m.TreeView.Items[m.TreeView.Cursor]
	return m, m.dispatch(protocol.CommandWindowSwap{
		SessionID:    it.sessionID,
		WindowID:     w1,
		WithWindowID: w2,
	})
}

func (m Model) treeSelectCurrent() (Model, tea.Cmd) {
	if m.TreeView.Cursor < 0 || m.TreeView.Cursor >= len(m.TreeView.Items) {
		return m.exitTreeView(), nil
	}
	it := m.TreeView.Items[m.TreeView.Cursor]
	m = m.exitTreeView()
	switch it.kind {
	case treeItemSession:
		if it.sessionID != m.SessionID {
			m = m.applyTreeSession(it.sessionID)
		}
		return m, m.currentWindowResizeCmd()
	case treeItemWindow:
		if it.sessionID != m.SessionID {
			m = m.applyTreeSession(it.sessionID)
		}
		m = m.switchWindow(it.windowID)
		if snap, ok := m.Layouts[it.windowID]; ok && len(snap.Panes) > 0 {
			m.ActivePaneID = normalizeActivePane(m.ActivePaneID, snap.Panes)
			m.Layout.ActivePane = m.ActivePaneID
		}
		return m, m.currentWindowResizeCmd()
	case treeItemPane:
		if it.sessionID != m.SessionID {
			m = m.applyTreeSession(it.sessionID)
		}
		m = m.switchWindow(it.windowID)
		return m.startPaneFocusTarget(it.paneID)
	}
	return m, nil
}

func (m Model) treeKillCurrent() (Model, tea.Cmd) {
	if m.TreeView.Cursor < 0 || m.TreeView.Cursor >= len(m.TreeView.Items) {
		return m, nil
	}
	it := m.TreeView.Items[m.TreeView.Cursor]
	switch it.kind {
	case treeItemPane:
		m = m.exitTreeView()
		return m.startPaneClose(it.paneID)
	case treeItemWindow:
		m = m.exitTreeView()
		if it.windowID == m.WindowID {
			return m.startWindowClose()
		}
		return m, m.dispatch(protocol.CommandKillWindow{
			SessionID: it.sessionID,
			WindowID:  it.windowID,
		})
	case treeItemSession:
		name := m.treeSessionName(it.sessionID)
		m = m.exitTreeView()
		return m, m.dispatch(protocol.CommandKillSession{Name: name})
	}
	return m, nil
}

func (m Model) treeKillTagged() (Model, tea.Cmd) {
	if len(m.TreeView.Tagged) == 0 {
		return m, nil
	}
	var cmds []protocol.Command
	for _, it := range m.TreeView.Items {
		if !m.TreeView.Tagged[it.id] {
			continue
		}
		switch it.kind {
		case treeItemPane:
			cmds = append(cmds, protocol.CommandPaneClose{
				SessionID: it.sessionID,
				WindowID:  it.windowID,
				PaneID:    it.paneID,
			})
		case treeItemWindow:
			cmds = append(cmds, protocol.CommandKillWindow{
				SessionID: it.sessionID,
				WindowID:  it.windowID,
			})
		case treeItemSession:
			cmds = append(cmds, protocol.CommandKillSession{Name: m.treeSessionName(it.sessionID)})
		}
	}
	m = m.exitTreeView()
	if len(cmds) == 0 {
		return m, nil
	}
	if len(cmds) == 1 {
		return m, m.dispatch(cmds[0])
	}
	sup, ctx := m.Supervisor, m.Ctx
	if !sup.Valid() || ctx == nil {
		return m, nil
	}
	return m, func() tea.Msg {
		for _, cmd := range cmds {
			_ = sup.Send(ctx, cmd)
		}
		return nil
	}
}

func (m Model) applyTreeSession(sessionID protocol.SessionID) Model {
	return m.ApplySessionSnapshot(SessionSnapshotFromTree(m.TreeView.Snapshot, sessionID))
}

func (m Model) drawTreeView(canvas *runeCanvas) {
	if !m.TreeView.Open {
		return
	}
	cols, rows := canvas.cols, canvas.rows
	if cols < 1 || rows < 1 {
		return
	}
	previewRows := rows / 3
	if previewRows < 3 {
		previewRows = 3
	}
	if previewRows > rows-4 {
		previewRows = rows / 2
	}
	listRows := rows - previewRows - 1
	if listRows < 1 {
		listRows = rows - 1
		previewRows = 0
	}

	title := m.treeTitleLine()
	for x, r := range title {
		if x >= cols {
			break
		}
		canvas.set(x, 0, r)
	}

	start := 0
	if m.TreeView.Cursor >= listRows-1 {
		start = m.TreeView.Cursor - listRows + 2
	}
	for row := 0; row < listRows-1; row++ {
		idx := start + row
		y := row + 1
		if idx >= len(m.TreeView.Items) {
			break
		}
		line := m.treeItemLine(m.TreeView.Items[idx], cols)
		highlight := idx == m.TreeView.Cursor
		marked := m.treeIsMarked(m.TreeView.Items[idx])
		for x, r := range line {
			if x >= cols {
				break
			}
			switch {
			case highlight:
				canvas.setStyled(x, y, r, treeModeHighlightANSI)
			case marked:
				canvas.setStyled(x, y, r, treeModeMarkedANSI)
			default:
				canvas.set(x, y, r)
			}
		}
	}

	if m.TreeView.PromptKind != treePromptNone {
		prompt := m.treePromptLine()
		y := listRows
		if y >= rows {
			y = rows - 1
		}
		for x, r := range prompt {
			if x >= cols {
				break
			}
			canvas.set(x, y, r)
		}
	}

	if previewRows > 0 {
		previewY := listRows + 1
		for x := 0; x < cols; x++ {
			canvas.set(x, previewY-1, '─')
		}
		m.drawTreePreview(canvas, previewY, previewRows, cols)
	}

	canvas.drawOverlayStatus(m.copyModeOverlayANSI(), treeModeHelp)
}

func (m Model) treeTitleLine() string {
	var b strings.Builder
	b.WriteString("Tree view")
	if m.TreeView.Filter != "" {
		b.WriteString(" (filter: ")
		b.WriteString(m.TreeView.Filter)
		b.WriteByte(')')
	}
	if m.TreeView.SearchQuery != "" {
		b.WriteString(" (search: ")
		b.WriteString(m.TreeView.SearchQuery)
		b.WriteByte(')')
	}
	sortLabel := "index"
	if m.TreeView.SortOrder == treeSortName {
		sortLabel = "name"
	}
	b.WriteString(" [sort: ")
	b.WriteString(sortLabel)
	if m.TreeView.SortReversed {
		b.WriteString(", reversed")
	}
	b.WriteByte(']')
	return b.String()
}

func (m Model) treePromptLine() string {
	switch m.TreeView.PromptKind {
	case treePromptFilter:
		return "(filter) " + m.TreeView.PromptInput
	case treePromptSearch:
		return "(search) " + m.TreeView.PromptInput
	default:
		return ""
	}
}

func (m Model) treeItemLine(it treeItem, maxWidth int) string {
	var b strings.Builder
	if it.shortcutKey != "" {
		b.WriteByte('(')
		b.WriteString(it.shortcutKey)
		b.WriteString(") ")
	} else {
		b.WriteString("    ")
	}
	if it.depth == 0 {
		b.WriteString(m.treeExpandSymbol(it))
	} else {
		for d := 1; d < it.depth; d++ {
			if d-1 < len(it.ancestorsLast) && it.ancestorsLast[d-1] {
				b.WriteString("    ")
			} else {
				b.WriteString("│   ")
			}
		}
		if it.lastSibling {
			b.WriteString("└─ ")
		} else {
			b.WriteString("├─ ")
		}
		b.WriteString(m.treeExpandSymbol(it))
	}
	if it.tagged {
		b.WriteByte('*')
	}
	b.WriteString(it.label)
	if it.detail != "" {
		b.WriteString(": ")
		b.WriteString(it.detail)
	}
	line := b.String()
	if len(line) > maxWidth {
		if maxWidth > 1 {
			line = line[:maxWidth-1] + "…"
		} else {
			line = line[:maxWidth]
		}
	}
	return line
}

func (m Model) treeExpandSymbol(it treeItem) string {
	if !it.hasChildren {
		if it.depth > 0 {
			return "  "
		}
		return ""
	}
	if it.expanded {
		return "- "
	}
	return "+ "
}

func (m Model) drawTreePreview(canvas *runeCanvas, startY, height, cols int) {
	if m.TreeView.Cursor < 0 || m.TreeView.Cursor >= len(m.TreeView.Items) {
		return
	}
	it := m.TreeView.Items[m.TreeView.Cursor]
	if it.kind == treeItemSession {
		m.drawTreeSessionStrip(canvas, startY, height, cols, it)
		return
	}
	header := fmtPreviewHeader(it)
	for x, r := range header {
		if x >= cols {
			break
		}
		canvas.set(x, startY, r)
	}

	screen := m.treePreviewScreen(it)
	if screen == nil {
		msg := "(no preview)"
		for x, r := range msg {
			if x >= cols {
				break
			}
			canvas.set(x, startY+1, r)
		}
		return
	}
	lines := screen.Lines
	startLine := 0
	if len(lines) > height-1 {
		startLine = len(lines) - (height - 1)
	}
	for row := 0; row < height-1; row++ {
		y := startY + 1 + row
		if y >= canvas.rows {
			break
		}
		lineIdx := startLine + row
		if lineIdx >= len(lines) {
			break
		}
		line := stripANSI(screenLineText(lines[lineIdx]))
		for x, r := range line {
			if x >= cols {
				break
			}
			canvas.set(x, y, r)
		}
	}
}

func (m Model) drawTreeSessionStrip(canvas *runeCanvas, startY, height, cols int, it treeItem) {
	var sess *TreeSessionNode
	for i := range m.TreeView.Snapshot.Sessions {
		if m.TreeView.Snapshot.Sessions[i].SessionID == it.sessionID {
			sess = &m.TreeView.Snapshot.Sessions[i]
			break
		}
	}
	if sess == nil || len(sess.Windows) == 0 {
		msg := "Session: " + it.label
		for x, r := range msg {
			if x >= cols {
				break
			}
			canvas.set(x, startY, r)
		}
		return
	}
	windows := sess.Windows
	total := len(windows)
	visible := total
	minWidth := 24
	if cols/minWidth < visible {
		visible = cols / minWidth
		if visible < 1 {
			visible = 1
		}
	}
	current := 0
	for i, win := range windows {
		if win.WindowID == m.WindowID && it.sessionID == m.SessionID {
			current = i
			break
		}
	}
	start, end := 0, visible
	if current >= visible {
		if current >= total-visible {
			start = total - visible
			end = total
		} else {
			start = current - visible/2
			end = start + visible
		}
	}
	start += m.TreeView.PreviewOffset
	end += m.TreeView.PreviewOffset
	if start < 0 {
		end -= start
		start = 0
	}
	if end > total {
		start -= end - total
		end = total
	}
	if start < 0 {
		start = 0
	}
	left := start > 0
	right := end < total
	inner := cols
	if left {
		inner -= 3
	}
	if right {
		inner -= 3
	}
	each := inner / visible
	if each < 1 {
		each = 1
	}
	y := startY
	header := fmt.Sprintf("Session: %s", it.label)
	for x, r := range header {
		if x >= cols {
			break
		}
		canvas.set(x, y, r)
	}
	previewY := y + 1
	if previewY >= canvas.rows {
		return
	}
	xOff := 0
	if left {
		canvas.set(0, previewY+height/2, '<')
		for row := previewY; row < previewY+height-1 && row < canvas.rows; row++ {
			canvas.set(1, row, '│')
		}
		xOff = 3
	}
	for i := start; i < end; i++ {
		win := windows[i]
		slot := i - start
		xStart := xOff + slot*each
		label := win.Name
		if label == "" {
			label = string(win.WindowID)
		}
		if len(label) > each-1 {
			label = label[:each-1]
		}
		for x, r := range label {
			if xStart+x >= cols {
				break
			}
			canvas.set(xStart+x, previewY, r)
		}
		screen := m.treeWindowPreviewScreen(win.WindowID)
		if screen == nil {
			continue
		}
		lines := screen.Lines
		for row := 0; row < height-2; row++ {
			yy := previewY + 1 + row
			if yy >= canvas.rows {
				break
			}
			if row >= len(lines) {
				break
			}
			line := stripANSI(screenLineText(lines[row]))
			for x, r := range line {
				if xStart+x >= cols {
					break
				}
				canvas.set(xStart+x, yy, r)
			}
		}
	}
	if right {
		rx := cols - 1
		canvas.set(rx, previewY+height/2, '>')
		for row := previewY; row < previewY+height-1 && row < canvas.rows; row++ {
			if cols >= 2 {
				canvas.set(rx-1, row, '│')
			}
		}
	}
}

func (m Model) treeWindowPreviewScreen(wid protocol.WindowID) *protocol.EventPaneScreenChanged {
	if screens := m.WindowScreens[wid]; screens != nil {
		for _, s := range screens {
			return &s
		}
	}
	if screens := m.TreeView.Snapshot.Screens[wid]; screens != nil {
		for _, s := range screens {
			return &s
		}
	}
	return nil
}

func fmtPreviewHeader(it treeItem) string {
	switch it.kind {
	case treeItemSession:
		return "Session: " + it.label
	case treeItemWindow:
		return "Window: " + it.label
	case treeItemPane:
		return "Pane: " + it.label
	default:
		return ""
	}
}

func (m Model) treePreviewScreen(it treeItem) *protocol.EventPaneScreenChanged {
	switch it.kind {
	case treeItemPane:
		if screens := m.WindowScreens[it.windowID]; screens != nil {
			if s, ok := screens[it.paneID]; ok {
				return &s
			}
		}
		if screens := m.TreeView.Snapshot.Screens[it.windowID]; screens != nil {
			if s, ok := screens[it.paneID]; ok {
				return &s
			}
		}
	case treeItemWindow:
		return m.treeWindowPreviewScreen(it.windowID)
	case treeItemSession:
		for _, wid := range m.WindowIDs {
			if screens := m.WindowScreens[wid]; screens != nil {
				for _, s := range screens {
					return &s
				}
			}
		}
	}
	return nil
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	esc := false
	for i := 0; i < len(s); i++ {
		if esc {
			if s[i] == 'm' {
				esc = false
			}
			continue
		}
		if s[i] == '\x1b' {
			esc = true
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
