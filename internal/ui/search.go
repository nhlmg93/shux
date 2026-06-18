package ui

import (
	"strings"
	"unicode/utf8"

	"shux/internal/protocol"
)

const (
	searchMaxQueryRunes = 128
	searchMaxRows       = 4096
	searchMaxMatches    = 256
)

type searchDirection int8

const (
	searchDirectionForward searchDirection = 1
	searchDirectionReverse searchDirection = -1
)

type searchMatch struct {
	Row int
	Col int
	Len int
}

type searchState struct {
	Active       bool
	Direction    searchDirection
	LastDir      searchDirection
	Query        string
	PaneID       protocol.PaneID
	Matches      []searchMatch
	ActiveIndex  int
	MatchLimited bool
}

func newSearchState() searchState {
	return searchState{
		ActiveIndex: -1,
	}
}

func (m *Model) beginSearch(direction searchDirection) {
	m.Search.Active = true
	m.Search.Direction = direction
	m.Search.PaneID = m.ActivePaneID
}

func (m *Model) clearSearchQuery() {
	m.Search.Query = ""
	m.Search.Matches = nil
	m.Search.ActiveIndex = -1
	m.Search.MatchLimited = false
}

func (m *Model) commitSearch() {
	m.Search.Active = false
	m.Search.PaneID = m.ActivePaneID
	m.Search.LastDir = m.Search.Direction
	m.refreshSearchMatchesForPane(m.Search.PaneID)
}

func (m *Model) refreshSearchMatchesForPane(paneID protocol.PaneID) {
	if !paneID.Valid() || m.Search.Query == "" || m.Search.PaneID != paneID {
		return
	}
	screen := m.paneScreen(paneID)
	matches, limited := findSearchMatches(screen.Lines, m.Search.Query, searchMaxRows, searchMaxMatches)
	m.Search.Matches = matches
	m.Search.MatchLimited = limited
	if len(matches) == 0 {
		m.Search.ActiveIndex = -1
		return
	}
	if m.Search.ActiveIndex >= 0 && m.Search.ActiveIndex < len(matches) {
		return
	}
	m.Search.ActiveIndex = initialMatchIndex(m.Search.LastDir, len(matches))
}

func (m *Model) moveSearchSelection(reverse bool) bool {
	if !m.CopyMode || len(m.Search.Matches) == 0 {
		return false
	}
	dir := m.Search.LastDir
	if dir == 0 {
		dir = searchDirectionForward
	}
	if reverse {
		dir = oppositeDirection(dir)
	}
	step := 1
	if dir == searchDirectionReverse {
		step = -1
	}
	idx := m.Search.ActiveIndex
	if idx < 0 || idx >= len(m.Search.Matches) {
		idx = initialMatchIndex(dir, len(m.Search.Matches))
	} else {
		idx = wrapIndex(idx+step, len(m.Search.Matches))
	}
	m.Search.ActiveIndex = idx
	return true
}

func (m Model) searchOverlayForPane(paneID protocol.PaneID) paneSearchOverlay {
	if !m.CopyMode || m.Search.Query == "" || paneID != m.Search.PaneID || len(m.Search.Matches) == 0 {
		return paneSearchOverlay{}
	}
	overlay := paneSearchOverlay{
		matches: make(map[searchCell]struct{}),
		active:  make(map[searchCell]struct{}),
	}
	for i, match := range m.Search.Matches {
		target := overlay.matches
		if i == m.Search.ActiveIndex {
			target = overlay.active
		}
		for col := 0; col < match.Len; col++ {
			target[searchCell{row: match.Row, col: match.Col + col}] = struct{}{}
		}
	}
	return overlay
}

func findSearchMatches(lines []protocol.EventPaneScreenLine, query string, maxRows, maxMatches int) ([]searchMatch, bool) {
	if maxRows <= 0 || maxMatches <= 0 {
		return nil, false
	}
	needle := strings.ToLower(query)
	if needle == "" {
		return nil, false
	}
	start := 0
	if len(lines) > maxRows {
		start = len(lines) - maxRows
	}
	matches := make([]searchMatch, 0, min(len(lines), maxMatches))
	matchLen := utf8.RuneCountInString(query)
	limited := false
	for row := start; row < len(lines); row++ {
		text := screenLineText(lines[row])
		if text == "" {
			continue
		}
		lower := strings.ToLower(text)
		cursor := 0
		for cursor < len(lower) {
			idx := strings.Index(lower[cursor:], needle)
			if idx < 0 {
				break
			}
			byteIndex := cursor + idx
			matches = append(matches, searchMatch{
				Row: row - start,
				Col: utf8.RuneCountInString(text[:byteIndex]),
				Len: matchLen,
			})
			if len(matches) >= maxMatches {
				limited = true
				return matches, limited
			}
			cursor = byteIndex + len(needle)
		}
	}
	return matches, limited
}

func screenLineText(line protocol.EventPaneScreenLine) string {
	if line.Text != "" {
		return line.Text
	}
	if len(line.Cells) == 0 {
		return ""
	}
	var b strings.Builder
	for _, cell := range line.Cells {
		if cell.Text == "" {
			b.WriteByte(' ')
			continue
		}
		b.WriteString(cell.Text)
	}
	return b.String()
}

func initialMatchIndex(dir searchDirection, total int) int {
	if total <= 0 {
		return -1
	}
	if dir == searchDirectionReverse {
		return total - 1
	}
	return 0
}

func oppositeDirection(dir searchDirection) searchDirection {
	if dir == searchDirectionReverse {
		return searchDirectionForward
	}
	return searchDirectionReverse
}

func wrapIndex(index, total int) int {
	if total <= 0 {
		return -1
	}
	index %= total
	if index < 0 {
		index += total
	}
	return index
}

func trimToRunes(s string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes])
}
