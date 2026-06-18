package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"shux/internal/protocol"
)

// TreeSnapshotData is the full session/window/pane hierarchy for tree mode.
type TreeSnapshotData struct {
	Sessions []TreeSessionNode
	Screens  map[protocol.WindowID]map[protocol.PaneID]protocol.EventPaneScreenChanged
}

type TreeSessionNode struct {
	SessionID protocol.SessionID
	Name      string
	Attached  bool
	Windows   []TreeWindowNode
}

type TreeWindowNode struct {
	WindowID protocol.WindowID
	Index    int
	Name     string
	Panes    []TreePaneNode
	Layout   LayoutSnapshot
}

type TreePaneNode struct {
	PaneID protocol.PaneID
	Index  int
	Name   string
	Col    int
	Row    int
	Cols   int
	Rows   int
}

// TreeSnapshotProvider loads hierarchy data; set from shux attach bootstrap.
type TreeSnapshotProvider func(ctx context.Context, clientSession protocol.SessionID) (TreeSnapshotData, error)

type treeItemKind uint8

const (
	treeItemSession treeItemKind = iota
	treeItemWindow
	treeItemPane
)

type treeItemID struct {
	kind      treeItemKind
	sessionID protocol.SessionID
	windowID  protocol.WindowID
	paneID    protocol.PaneID
}

func (id treeItemID) valid() bool {
	switch id.kind {
	case treeItemSession:
		return id.sessionID.Valid()
	case treeItemWindow:
		return id.sessionID.Valid() && id.windowID.Valid()
	case treeItemPane:
		return id.sessionID.Valid() && id.windowID.Valid() && id.paneID.Valid()
	}
	return false
}

type treeItem struct {
	id          treeItemID
	kind        treeItemKind
	sessionID   protocol.SessionID
	windowID    protocol.WindowID
	paneID      protocol.PaneID
	depth       int
	parent      int
	expanded    bool
	hasChildren bool
	lastSibling bool
	ancestorsLast []bool
	label       string
	detail      string
	tagged      bool
	shortcutKey string
}

type treeViewMode uint8

const (
	treeViewDefault treeViewMode = iota
	treeViewSessionsCollapsed
	treeViewWindowsCollapsed
)

type treePromptKind uint8

const (
	treePromptNone treePromptKind = iota
	treePromptFilter
	treePromptSearch
)

type treeSortOrder uint8

const (
	treeSortIndex treeSortOrder = iota
	treeSortName
)

type TreeView struct {
	Open          bool
	Mode          treeViewMode
	Snapshot      TreeSnapshotData
	Items         []treeItem
	Cursor        int
	PromptKind    treePromptKind
	PromptInput   string
	SearchQuery   string
	SearchForward bool
	Filter        string
	SortOrder     treeSortOrder
	SortReversed  bool
	PreviewOffset int
	Tagged        map[treeItemID]bool
	Marked        treeItemID
}

func (m Model) startTreeView(mode treeViewMode) (Model, tea.Cmd) {
	m.TreeView.Open = true
	m.TreeView.Mode = mode
	m.TreeView.Cursor = 0
	if m.TreeView.Tagged == nil {
		m.TreeView.Tagged = make(map[treeItemID]bool)
	}
	if m.TreeSnapshot != nil && m.Ctx != nil {
		ctx := m.Ctx
		provider := m.TreeSnapshot
		return m, func() tea.Msg {
			data, err := provider(ctx, m.SessionID)
			return treeSnapshotMsg{Data: data, Err: err}
		}
	}
	m.TreeView.Snapshot = m.treeSnapshotFromModel()
	m.rebuildTreeItems()
	m.TreeView.Cursor = m.treeCursorForCurrent()
	return m, nil
}

func (m Model) exitTreeView() Model {
	m.TreeView = TreeView{}
	return m
}

type treeSnapshotMsg struct {
	Data TreeSnapshotData
	Err  error
}

func (m Model) treeSnapshotFromModel() TreeSnapshotData {
	sess := TreeSessionNode{
		SessionID: m.SessionID,
		Name:      string(m.SessionID),
		Attached:  true,
	}
	for i, wid := range m.WindowIDs {
		if m.ClosedWindows[wid] {
			continue
		}
		win := TreeWindowNode{
			WindowID: wid,
			Index:    i + 1,
			Name:     m.treeWindowName(wid),
		}
		layout, ok := m.Layouts[wid]
		if !ok {
			win.Panes = nil
			sess.Windows = append(sess.Windows, win)
			continue
		}
		for pi, p := range layout.Panes {
			win.Panes = append(win.Panes, TreePaneNode{
				PaneID: p.PaneID,
				Index:  pi + 1,
				Name:   m.paneName(wid, p.PaneID),
				Col:    p.Col,
				Row:    p.Row,
				Cols:   p.Cols,
				Rows:   p.Rows,
			})
		}
		win.Layout = layout
		sess.Windows = append(sess.Windows, win)
	}
	return TreeSnapshotData{Sessions: []TreeSessionNode{sess}}
}

func (m Model) treeWindowName(wid protocol.WindowID) string {
	if name := strings.TrimSpace(m.WindowNames[wid]); name != "" {
		return name
	}
	return string(wid)
}

func (m *Model) rebuildTreeItems() {
	collapseSessions := m.TreeView.Mode == treeViewSessionsCollapsed
	collapseWindows := m.TreeView.Mode == treeViewWindowsCollapsed

	expandedSessions := make(map[protocol.SessionID]bool)
	expandedWindows := make(map[protocol.WindowID]bool)
	for _, it := range m.TreeView.Items {
		switch it.kind {
		case treeItemSession:
			if it.expanded {
				expandedSessions[it.sessionID] = true
			}
		case treeItemWindow:
			if it.expanded {
				expandedWindows[it.windowID] = true
			}
		}
	}

	sessions := append([]TreeSessionNode(nil), m.TreeView.Snapshot.Sessions...)
	type indexedSession struct {
		node  TreeSessionNode
		index int
	}
	indexed := make([]indexedSession, len(sessions))
	for i, sess := range sessions {
		indexed[i] = indexedSession{node: sess, index: i + 1}
	}
	sort.SliceStable(indexed, func(i, j int) bool {
		a, b := indexed[i], indexed[j]
		less := m.treeSortLess(a.node.Name, string(a.node.SessionID), a.index, b.node.Name, string(b.node.SessionID), b.index)
		if m.TreeView.SortReversed {
			return !less
		}
		return less
	})

	var items []treeItem
	for si, entry := range indexed {
		sess := entry.node
		sessExpanded := !collapseSessions
		if v, ok := expandedSessions[sess.SessionID]; ok {
			sessExpanded = v
		}
		windows := append([]TreeWindowNode(nil), sess.Windows...)
		m.sortTreeWindows(windows)
		if !m.treeNodeMatchesFilter(sess.Name, fmt.Sprintf("%d windows", len(windows))) {
			if !m.treeSessionHasFilterMatch(sess, windows) {
				continue
			}
		}
		siIdx := len(items)
		winCount := len(windows)
		detail := fmt.Sprintf("%d windows", winCount)
		if sess.Attached {
			detail += " (attached)"
		}
		label := sess.Name
		if label == "" {
			label = string(sess.SessionID)
		}
		sid := treeItemID{kind: treeItemSession, sessionID: sess.SessionID}
		items = append(items, treeItem{
			id:          sid,
			kind:        treeItemSession,
			sessionID:   sess.SessionID,
			depth:       0,
			parent:      -1,
			expanded:    sessExpanded,
			hasChildren: winCount > 0,
			lastSibling: si == len(indexed)-1,
			label:       label,
			detail:      detail,
			tagged:      m.TreeView.Tagged[sid],
		})
		if !sessExpanded {
			continue
		}
		for wi, win := range windows {
			winExpanded := !collapseWindows
			if v, ok := expandedWindows[win.WindowID]; ok {
				winExpanded = v
			}
			panes := append([]TreePaneNode(nil), win.Panes...)
			m.sortTreePanes(panes)
			if !m.treeNodeMatchesFilter(win.Name, fmt.Sprintf("#%d", win.Index)) {
				if !m.treeWindowHasFilterMatch(win, panes) {
					continue
				}
			}
			wiIdx := len(items)
			paneCount := len(panes)
			winLabel := win.Name
			if winLabel == "" {
				winLabel = string(win.WindowID)
			}
			wid := treeItemID{kind: treeItemWindow, sessionID: sess.SessionID, windowID: win.WindowID}
			items = append(items, treeItem{
				id:            wid,
				kind:          treeItemWindow,
				sessionID:     sess.SessionID,
				windowID:      win.WindowID,
				depth:         1,
				parent:        siIdx,
				expanded:      winExpanded,
				hasChildren:   paneCount > 0,
				lastSibling:   wi == len(windows)-1,
				ancestorsLast: []bool{items[siIdx].lastSibling},
				label:         winLabel,
				detail:        fmt.Sprintf("#%d, %d panes", win.Index, paneCount),
				tagged:        m.TreeView.Tagged[wid],
			})
			if !winExpanded {
				continue
			}
			for pi, pane := range panes {
				if !m.treeNodeMatchesFilter(pane.Name, fmt.Sprintf("#%d", pane.Index)) {
					continue
				}
				paneLabel := pane.Name
				if paneLabel == "" {
					paneLabel = string(pane.PaneID)
				}
				pid := treeItemID{kind: treeItemPane, sessionID: sess.SessionID, windowID: win.WindowID, paneID: pane.PaneID}
				items = append(items, treeItem{
					id:            pid,
					kind:          treeItemPane,
					sessionID:     sess.SessionID,
					windowID:      win.WindowID,
					paneID:        pane.PaneID,
					depth:         2,
					parent:        wiIdx,
					lastSibling:   pi == len(panes)-1,
					ancestorsLast: []bool{items[siIdx].lastSibling, items[wiIdx].lastSibling},
					label:         paneLabel,
					detail:        fmt.Sprintf("#%d", pane.Index),
					tagged:        m.TreeView.Tagged[pid],
				})
			}
		}
	}
	for i := range items {
		items[i].shortcutKey = treeShortcutKey(i)
	}
	m.TreeView.Items = items
	if m.TreeView.Cursor >= len(items) {
		if len(items) > 0 {
			m.TreeView.Cursor = len(items) - 1
		} else {
			m.TreeView.Cursor = 0
		}
	}
}

func treeShortcutKey(line int) string {
	switch {
	case line < 10:
		return fmt.Sprintf("%d", line)
	case line < 36:
		return fmt.Sprintf("M-%c", 'a'+line-10)
	default:
		return ""
	}
}


func (m Model) sortTreeWindows(windows []TreeWindowNode) {
	sort.SliceStable(windows, func(i, j int) bool {
		less := m.treeSortLess(windows[i].Name, string(windows[i].WindowID), windows[i].Index, windows[j].Name, string(windows[j].WindowID), windows[j].Index)
		if m.TreeView.SortReversed {
			return !less
		}
		return less
	})
}

func (m Model) sortTreePanes(panes []TreePaneNode) {
	sort.SliceStable(panes, func(i, j int) bool {
		less := m.treeSortLess(panes[i].Name, string(panes[i].PaneID), panes[i].Index, panes[j].Name, string(panes[j].PaneID), panes[j].Index)
		if m.TreeView.SortReversed {
			return !less
		}
		return less
	})
}

func (m Model) treeSortLess(nameA, fallbackA string, indexA int, nameB, fallbackB string, indexB int) bool {
	switch m.TreeView.SortOrder {
	case treeSortName:
		a := strings.ToLower(strings.TrimSpace(nameA))
		if a == "" {
			a = strings.ToLower(fallbackA)
		}
		b := strings.ToLower(strings.TrimSpace(nameB))
		if b == "" {
			b = strings.ToLower(fallbackB)
		}
		return a < b
	default:
		return indexA < indexB
	}
}

func (m Model) treeNodeMatchesFilter(parts ...string) bool {
	filter := strings.TrimSpace(m.TreeView.Filter)
	if filter == "" {
		return true
	}
	needle := strings.ToLower(filter)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(strings.ToLower(part), needle) {
			return true
		}
	}
	return false
}

func (m Model) treeSessionHasFilterMatch(sess TreeSessionNode, windows []TreeWindowNode) bool {
	for _, win := range windows {
		if m.treeWindowHasFilterMatch(win, win.Panes) {
			return true
		}
	}
	return m.treeNodeMatchesFilter(sess.Name, string(sess.SessionID))
}

func (m Model) treeWindowHasFilterMatch(win TreeWindowNode, panes []TreePaneNode) bool {
	for _, pane := range panes {
		if m.treeNodeMatchesFilter(pane.Name, string(pane.PaneID), fmt.Sprintf("#%d", pane.Index)) {
			return true
		}
	}
	return m.treeNodeMatchesFilter(win.Name, string(win.WindowID), fmt.Sprintf("#%d", win.Index))
}

func (m Model) treeCursorForCurrent() int {
	for i, it := range m.TreeView.Items {
		switch it.kind {
		case treeItemPane:
			if it.sessionID == m.SessionID && it.windowID == m.WindowID && it.paneID == m.ActivePaneID {
				return i
			}
		case treeItemWindow:
			if it.sessionID == m.SessionID && it.windowID == m.WindowID {
				return i
			}
		case treeItemSession:
			if it.sessionID == m.SessionID {
				return i
			}
		}
	}
	return 0
}

func (m Model) treeExpandCurrent() Model {
	if m.TreeView.Cursor < 0 || m.TreeView.Cursor >= len(m.TreeView.Items) {
		return m
	}
	it := &m.TreeView.Items[m.TreeView.Cursor]
	if it.hasChildren {
		it.expanded = true
	}
	m.rebuildTreeItems()
	m.TreeView.Cursor = m.treeIndexOf(it.sessionID, it.windowID, it.paneID, it.kind)
	return m
}

func (m Model) treeCollapseCurrent() Model {
	if m.TreeView.Cursor < 0 || m.TreeView.Cursor >= len(m.TreeView.Items) {
		return m
	}
	it := m.TreeView.Items[m.TreeView.Cursor]
	if it.expanded && it.hasChildren {
		m.TreeView.Items[m.TreeView.Cursor].expanded = false
		m.rebuildTreeItems()
		m.TreeView.Cursor = m.treeIndexOf(it.sessionID, it.windowID, it.paneID, it.kind)
		return m
	}
	if it.parent >= 0 {
		m.TreeView.Cursor = it.parent
	}
	return m
}

func (m Model) treeIndexOf(sid protocol.SessionID, wid protocol.WindowID, pid protocol.PaneID, kind treeItemKind) int {
	for i, it := range m.TreeView.Items {
		if it.kind != kind {
			continue
		}
		switch kind {
		case treeItemSession:
			if it.sessionID == sid {
				return i
			}
		case treeItemWindow:
			if it.sessionID == sid && it.windowID == wid {
				return i
			}
		case treeItemPane:
			if it.sessionID == sid && it.windowID == wid && it.paneID == pid {
				return i
			}
		}
	}
	return m.TreeView.Cursor
}

func (m Model) treeJumpToHere() Model {
	m = m.treeExpandToCurrent()
	m.TreeView.Cursor = m.treeCursorForCurrent()
	return m
}

func (m Model) treeExpandToCurrent() Model {
	for _, sess := range m.TreeView.Snapshot.Sessions {
		if sess.SessionID != m.SessionID {
			continue
		}
		for i := range m.TreeView.Items {
			if m.TreeView.Items[i].kind == treeItemSession && m.TreeView.Items[i].sessionID == m.SessionID {
				m.TreeView.Items[i].expanded = true
			}
		}
		for _, win := range sess.Windows {
			if win.WindowID != m.WindowID {
				continue
			}
			for i := range m.TreeView.Items {
				if m.TreeView.Items[i].kind == treeItemWindow && m.TreeView.Items[i].windowID == m.WindowID {
					m.TreeView.Items[i].expanded = true
				}
			}
		}
	}
	m.rebuildTreeItems()
	return m
}

func (m Model) treeItemMatchesSearch(it treeItem, query string) bool {
	if query == "" {
		return false
	}
	icase := treeSearchCaseInsensitive(query)
	targets := []string{it.label, it.detail}
	switch it.kind {
	case treeItemSession:
		targets = append(targets, string(it.sessionID))
	case treeItemWindow:
		targets = append(targets, string(it.windowID))
	case treeItemPane:
		targets = append(targets, string(it.paneID))
	}
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		if icase {
			if strings.Contains(strings.ToLower(target), strings.ToLower(query)) {
				return true
			}
		} else if strings.Contains(target, query) {
			return true
		}
	}
	return false
}

func treeSearchCaseInsensitive(query string) bool {
	for _, r := range query {
		if r >= 'A' && r <= 'Z' {
			return false
		}
	}
	return true
}

func (m Model) treeSearchFrom(cursor int, forward bool) int {
	if m.TreeView.SearchQuery == "" || len(m.TreeView.Items) == 0 {
		return cursor
	}
	n := len(m.TreeView.Items)
	for step := 1; step <= n; step++ {
		idx := cursor
		if forward {
			idx = (cursor + step) % n
		} else {
			idx = (cursor - step + n) % n
		}
		if m.treeItemMatchesSearch(m.TreeView.Items[idx], m.TreeView.SearchQuery) {
			return idx
		}
	}
	return cursor
}

func (m Model) treeSessionName(sessionID protocol.SessionID) string {
	for _, sess := range m.TreeView.Snapshot.Sessions {
		if sess.SessionID == sessionID {
			if name := strings.TrimSpace(sess.Name); name != "" {
				return name
			}
			return string(sess.SessionID)
		}
	}
	return string(sessionID)
}

func (m Model) treeAdjacentWindow(cursor, direction int) (protocol.WindowID, protocol.WindowID, bool) {
	if cursor < 0 || cursor >= len(m.TreeView.Items) {
		return "", "", false
	}
	it := m.TreeView.Items[cursor]
	if it.kind != treeItemWindow {
		return "", "", false
	}
	var windows []protocol.WindowID
	for _, item := range m.TreeView.Items {
		if item.kind == treeItemWindow && item.sessionID == it.sessionID {
			windows = append(windows, item.windowID)
		}
	}
	if len(windows) < 2 {
		return "", "", false
	}
	idx := -1
	for i, wid := range windows {
		if wid == it.windowID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", "", false
	}
	other := idx + direction
	if other < 0 || other >= len(windows) {
		return "", "", false
	}
	return it.windowID, windows[other], true
}

func (m Model) treeUpdateSessionWindows(sessionID protocol.SessionID, windows []protocol.WindowID) Model {
	for si := range m.TreeView.Snapshot.Sessions {
		sess := &m.TreeView.Snapshot.Sessions[si]
		if sess.SessionID != sessionID {
			continue
		}
		byID := make(map[protocol.WindowID]TreeWindowNode, len(sess.Windows))
		for _, win := range sess.Windows {
			byID[win.WindowID] = win
		}
		ordered := make([]TreeWindowNode, 0, len(windows))
		for i, wid := range windows {
			win, ok := byID[wid]
			if !ok {
				continue
			}
			win.Index = i + 1
			ordered = append(ordered, win)
		}
		sess.Windows = ordered
		break
	}
	return m
}

func (m Model) treeIsMarked(it treeItem) bool {
	if !m.TreeView.Marked.valid() {
		return false
	}
	switch it.kind {
	case treeItemPane:
		return m.TreeView.Marked.kind == treeItemPane &&
			m.TreeView.Marked.sessionID == it.sessionID &&
			m.TreeView.Marked.windowID == it.windowID &&
			m.TreeView.Marked.paneID == it.paneID
	case treeItemWindow:
		return m.TreeView.Marked.kind == treeItemWindow &&
			m.TreeView.Marked.sessionID == it.sessionID &&
			m.TreeView.Marked.windowID == it.windowID
	default:
		return false
	}
}
