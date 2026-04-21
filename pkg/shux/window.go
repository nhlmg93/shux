package shux

import (
	"fmt"

	"github.com/mitchellh/go-libghostty"
)

type WindowRef struct {
	*loopRef
}

func (r *WindowRef) Send(msg any) bool {
	if r == nil {
		return false
	}
	return r.send(msg)
}

func (r *WindowRef) Ask(msg any) chan any {
	if r == nil {
		return nil
	}
	return r.ask(msg)
}

func (r *WindowRef) Stop() {
	if r != nil {
		r.stopLoop()
	}
}

func (r *WindowRef) Shutdown() {
	if r != nil {
		r.shutdown()
	}
}

type paneLayout struct {
	paneID uint32
	row    int
	col    int
	rows   int
	cols   int
}

type splitNode struct {
	paneID uint32
	dir    SplitDir
	first  *splitNode
	second *splitNode
}

func leafNode(paneID uint32) *splitNode {
	return &splitNode{paneID: paneID}
}

func (n *splitNode) isLeaf() bool {
	return n != nil && n.first == nil && n.second == nil
}

type dividerSegment struct {
	horizontal bool
	row        int
	col        int
	length     int
}

type borderState struct {
	h      bool
	v      bool
	active bool
}

type Window struct {
	ref       *WindowRef
	parent    *SessionRef
	id        uint32
	panes     map[uint32]*PaneRef
	paneOrder []uint32
	active    uint32
	paneID    uint32

	root     *splitNode
	layout   []paneLayout
	splitDir SplitDir
	rows     int
	cols     int
}

func NewWindow(id uint32) *Window {
	return &Window{
		id:    id,
		panes: make(map[uint32]*PaneRef),
	}
}

func StartWindow(id uint32, parent *SessionRef) *WindowRef {
	w := NewWindow(id)
	w.parent = parent
	ref := &WindowRef{loopRef: newLoopRef(32)}
	w.ref = ref
	go w.run()
	return ref
}

func (w *Window) run() {
	var reason error
	defer func() {
		if r := recover(); r != nil {
			reason = fmt.Errorf("panic: %v", r)
		}
		w.terminate(reason)
		close(w.ref.done)
	}()

	for {
		select {
		case <-w.ref.stop:
			return
		case msg := <-w.ref.inbox:
			w.receive(msg)
		}
	}
}

func (w *Window) terminate(reason error) {
	for _, pane := range w.panes {
		if pane != nil {
			pane.Shutdown()
		}
	}
	if reason != nil {
		Errorf("window: crash id=%d reason=%v", w.id, reason)
		return
	}
	Infof("window: terminate id=%d", w.id)
}

func (w *Window) receive(msg any) {
	switch m := msg.(type) {
	case CreatePane:
		w.createPane(m)
	case RestoreWindowLayout:
		w.restoreWindowLayout(m.Root, m.ActivePane)
	case Split:
		w.splitPane(m.Dir)
	case NavigatePane:
		w.navigatePane(m.Dir)
	case SwitchToPane:
		w.switchToPane(m.Index)
	case PaneExited:
		w.handlePaneExited(m.ID)
	case PaneContentUpdated:
		if m.ID == 0 || m.ID == w.active {
			if w.parent != nil {
				w.parent.Send(m)
			}
		}
	case ResizeMsg:
		w.resizeAllPanes(m.Rows, m.Cols)
	case WriteToPane, KeyInput:
		if pane := w.activePane(); pane != nil {
			pane.Send(m)
		}
	case askEnvelope:
		w.handleAsk(m)
	}
}

func (w *Window) handleAsk(envelope askEnvelope) {
	switch envelope.msg.(type) {
	case GetActivePane:
		if pane := w.activePane(); pane != nil {
			envelope.reply <- pane
			return
		}
		envelope.reply <- nil
	case GetPaneContent:
		if pane := w.activePane(); pane != nil {
			result, _ := askValue(pane, envelope.msg)
			envelope.reply <- result
			return
		}
		envelope.reply <- nil
	case GetWindowView:
		envelope.reply <- w.buildWindowView()
	case GetWindowSnapshotData:
		envelope.reply <- w.gatherSnapshotData()
	default:
		envelope.reply <- nil
	}
}

func (w *Window) gatherSnapshotData() WindowSnapshot {
	snapshot := WindowSnapshot{
		ID:         w.id,
		ActivePane: w.active,
		PaneOrder:  append([]uint32(nil), w.paneOrder...),
		Panes:      make([]PaneSnapshot, 0, len(w.paneOrder)),
		Layout:     snapshotSplitTree(w.root),
	}

	for _, paneID := range w.paneOrder {
		paneRef, ok := w.panes[paneID]
		if !ok {
			continue
		}

		result, _ := askValue(paneRef, GetPaneSnapshotData{})
		paneData, ok := result.(PaneSnapshotData)
		if !ok {
			continue
		}

		snapshot.Panes = append(snapshot.Panes, PaneSnapshot(paneData))
	}

	return snapshot
}

func (w *Window) activePane() *PaneRef {
	if w.active == 0 {
		return nil
	}
	return w.panes[w.active]
}

func (w *Window) createPane(cmd CreatePane) {
	paneID := cmd.ID
	if paneID == 0 {
		w.paneID++
		paneID = w.paneID
	} else if paneID > w.paneID {
		w.paneID = paneID
	}

	ref := StartPane(paneID, cmd.Rows, cmd.Cols, cmd.Shell, cmd.CWD, w.ref)
	w.panes[paneID] = ref
	w.paneOrder = append(w.paneOrder, paneID)

	if w.active == 0 {
		w.active = paneID
		w.splitDir = SplitH
		w.rows = cmd.Rows
		w.cols = cmd.Cols
		w.root = leafNode(paneID)
	} else {
		w.root, _ = splitAroundPane(w.root, w.active, w.splitDir, paneID)
		w.active = paneID
	}

	w.syncLayout()
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
}

func (w *Window) splitPane(dir SplitDir) {
	if w.active == 0 || len(w.paneOrder) == 0 {
		return
	}

	w.splitDir = dir

	var (
		shell string
		cwd   string
	)
	if active := w.activePane(); active != nil {
		result, _ := askValue(active, GetPaneSnapshotData{})
		if snap, ok := result.(PaneSnapshotData); ok {
			shell = snap.Shell
			cwd = snap.CWD
		}
	}

	w.paneID++
	newPaneID := w.paneID
	newRef := StartPane(newPaneID, w.rows, w.cols, shell, cwd, w.ref)
	w.panes[newPaneID] = newRef
	w.paneOrder = append(w.paneOrder, newPaneID)
	w.root, _ = splitAroundPane(w.root, w.active, dir, newPaneID)
	w.active = newPaneID

	w.syncLayout()
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
}

func splitAroundPane(node *splitNode, target uint32, dir SplitDir, newPaneID uint32) (*splitNode, bool) {
	if node == nil {
		return nil, false
	}
	if node.isLeaf() {
		if node.paneID != target {
			return node, false
		}
		return &splitNode{
			dir:    dir,
			first:  leafNode(target),
			second: leafNode(newPaneID),
		}, true
	}
	if next, ok := splitAroundPane(node.first, target, dir, newPaneID); ok {
		node.first = next
		return node, true
	}
	if next, ok := splitAroundPane(node.second, target, dir, newPaneID); ok {
		node.second = next
		return node, true
	}
	return node, false
}

func snapshotSplitTree(node *splitNode) *SplitTreeSnapshot {
	if node == nil {
		return nil
	}
	if node.isLeaf() {
		return &SplitTreeSnapshot{PaneID: node.paneID}
	}
	return &SplitTreeSnapshot{
		Dir:    node.dir,
		First:  snapshotSplitTree(node.first),
		Second: snapshotSplitTree(node.second),
	}
}

func restoreSplitTree(node *SplitTreeSnapshot) *splitNode {
	if node == nil {
		return nil
	}
	if node.First == nil && node.Second == nil {
		return leafNode(node.PaneID)
	}
	return &splitNode{
		dir:    node.Dir,
		first:  restoreSplitTree(node.First),
		second: restoreSplitTree(node.Second),
	}
}

func collectTreePaneIDs(node *splitNode, out map[uint32]struct{}) {
	if node == nil {
		return
	}
	if node.isLeaf() {
		out[node.paneID] = struct{}{}
		return
	}
	collectTreePaneIDs(node.first, out)
	collectTreePaneIDs(node.second, out)
}

func (w *Window) restoreWindowLayout(root *SplitTreeSnapshot, activePane uint32) {
	if root == nil {
		return
	}
	restored := restoreSplitTree(root)
	if restored == nil {
		return
	}
	paneIDs := make(map[uint32]struct{})
	collectTreePaneIDs(restored, paneIDs)
	for paneID := range paneIDs {
		if _, ok := w.panes[paneID]; !ok {
			Warnf("window %d: restore layout references missing pane %d", w.id, paneID)
			return
		}
	}
	w.root = restored
	if activePane != 0 {
		if _, ok := w.panes[activePane]; ok {
			w.active = activePane
		}
	}
	w.syncLayout()
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
}

func (w *Window) switchToPane(index int) {
	if index < 0 || index >= len(w.paneOrder) {
		return
	}
	newActive := w.paneOrder[index]
	if newActive == w.active {
		return
	}
	w.active = newActive
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
}

func (w *Window) navigatePane(dir PaneNavDir) {
	activeLayout, ok := findLayout(w.layout, w.active)
	if !ok {
		return
	}

	bestID := uint32(0)
	bestPrimary := 0
	bestOrth := 0
	foundOverlap := false
	foundAny := false

	for _, cand := range w.layout {
		if cand.paneID == w.active {
			continue
		}

		primary, orth, overlaps, ok := paneNavMetrics(activeLayout, cand, dir)
		if !ok {
			continue
		}
		if !foundAny ||
			(foundOverlap != overlaps && overlaps) ||
			(foundOverlap == overlaps && (primary < bestPrimary || (primary == bestPrimary && orth < bestOrth))) {
			bestID = cand.paneID
			bestPrimary = primary
			bestOrth = orth
			foundOverlap = overlaps
			foundAny = true
		}
	}

	if foundAny && bestID != 0 && bestID != w.active {
		w.active = bestID
		if w.parent != nil {
			w.parent.Send(PaneContentUpdated{})
		}
	}
}

func paneNavMetrics(active, cand paneLayout, dir PaneNavDir) (primary int, orth int, overlaps bool, ok bool) {
	aRow0, aRow1 := active.row, active.row+active.rows
	aCol0, aCol1 := active.col, active.col+active.cols
	cRow0, cRow1 := cand.row, cand.row+cand.rows
	cCol0, cCol1 := cand.col, cand.col+cand.cols

	switch dir {
	case PaneNavLeft:
		primary = aCol0 - cCol1
		if primary < 0 {
			return 0, 0, false, false
		}
		orth = intervalGap(aRow0, aRow1, cRow0, cRow1)
		return primary, orth, orth == 0, true
	case PaneNavRight:
		primary = cCol0 - aCol1
		if primary < 0 {
			return 0, 0, false, false
		}
		orth = intervalGap(aRow0, aRow1, cRow0, cRow1)
		return primary, orth, orth == 0, true
	case PaneNavUp:
		primary = aRow0 - cRow1
		if primary < 0 {
			return 0, 0, false, false
		}
		orth = intervalGap(aCol0, aCol1, cCol0, cCol1)
		return primary, orth, orth == 0, true
	case PaneNavDown:
		primary = cRow0 - aRow1
		if primary < 0 {
			return 0, 0, false, false
		}
		orth = intervalGap(aCol0, aCol1, cCol0, cCol1)
		return primary, orth, orth == 0, true
	default:
		return 0, 0, false, false
	}
}

func intervalGap(a0, a1, b0, b1 int) int {
	if a1 <= b0 {
		return b0 - a1
	}
	if b1 <= a0 {
		return a0 - b1
	}
	return 0
}

func (w *Window) resizeAllPanes(rows, cols int) {
	Infof("window %d: resizing to %dx%d (was %dx%d)", w.id, rows, cols, w.rows, w.cols)
	w.rows = rows
	w.cols = cols
	w.syncLayout()
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
}

func (w *Window) handlePaneExited(id uint32) {
	currentIdx := w.activePaneIndex()
	delete(w.panes, id)
	w.paneOrder = removeOrderedID(w.paneOrder, id)

	w.root, _ = removePaneNode(w.root, id)

	if len(w.paneOrder) == 0 {
		w.active = 0
		w.layout = nil
		w.root = nil
		if w.parent != nil {
			w.parent.Send(WindowEmpty{ID: w.id})
		}
		return
	}

	if w.active == id {
		if currentIdx >= len(w.paneOrder) {
			currentIdx = len(w.paneOrder) - 1
		}
		if currentIdx < 0 {
			currentIdx = 0
		}
		w.active = w.paneOrder[currentIdx]
	}

	w.syncLayout()
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
}

func removePaneNode(node *splitNode, target uint32) (*splitNode, bool) {
	if node == nil {
		return nil, false
	}
	if node.isLeaf() {
		if node.paneID == target {
			return nil, true
		}
		return node, false
	}
	if next, removed := removePaneNode(node.first, target); removed {
		node.first = next
		if node.first == nil {
			return node.second, true
		}
		if node.second == nil {
			return node.first, true
		}
		return node, true
	}
	if next, removed := removePaneNode(node.second, target); removed {
		node.second = next
		if node.first == nil {
			return node.second, true
		}
		if node.second == nil {
			return node.first, true
		}
		return node, true
	}
	return node, false
}

func (w *Window) activePaneIndex() int {
	for i, id := range w.paneOrder {
		if id == w.active {
			return i
		}
	}
	return -1
}

func (w *Window) syncLayout() {
	if w.root == nil || w.rows <= 0 || w.cols <= 0 {
		w.layout = nil
		return
	}
	layouts := make([]paneLayout, 0, len(w.paneOrder))
	layoutSplitTree(w.root, 0, 0, w.rows, w.cols, &layouts)
	w.layout = layouts
	for _, pl := range w.layout {
		if paneRef, ok := w.panes[pl.paneID]; ok {
			Infof("window %d: resizing pane %d to %dx%d at %d,%d", w.id, pl.paneID, pl.rows, pl.cols, pl.row, pl.col)
			paneRef.Send(ResizeTerm{Rows: pl.rows, Cols: pl.cols})
		}
	}
}

func layoutSplitTree(node *splitNode, row, col, rows, cols int, out *[]paneLayout) {
	if node == nil {
		return
	}
	if node.isLeaf() {
		*out = append(*out, paneLayout{paneID: node.paneID, row: row, col: col, rows: rows, cols: cols})
		return
	}
	if node.dir == SplitH {
		firstRows, secondRows, ok := splitSpan(rows)
		if !ok {
			layoutSplitTree(node.first, row, col, rows, cols, out)
			return
		}
		layoutSplitTree(node.first, row, col, firstRows, cols, out)
		layoutSplitTree(node.second, row+firstRows+1, col, secondRows, cols, out)
		return
	}
	firstCols, secondCols, ok := splitSpan(cols)
	if !ok {
		layoutSplitTree(node.first, row, col, rows, cols, out)
		return
	}
	layoutSplitTree(node.first, row, col, rows, firstCols, out)
	layoutSplitTree(node.second, row, col+firstCols+1, rows, secondCols, out)
}

func splitSpan(total int) (first, second int, ok bool) {
	if total <= 1 {
		return total, 0, false
	}
	avail := total - 1
	first = (avail + 1) / 2
	second = avail - first
	return first, second, true
}

func findLayout(layouts []paneLayout, paneID uint32) (paneLayout, bool) {
	for _, pl := range layouts {
		if pl.paneID == paneID {
			return pl, true
		}
	}
	return paneLayout{}, false
}

func collectDividerSegments(node *splitNode, row, col, rows, cols int, out *[]dividerSegment) {
	if node == nil || node.isLeaf() {
		return
	}
	if node.dir == SplitH {
		firstRows, secondRows, ok := splitSpan(rows)
		if !ok {
			return
		}
		*out = append(*out, dividerSegment{horizontal: true, row: row + firstRows, col: col, length: cols})
		collectDividerSegments(node.first, row, col, firstRows, cols, out)
		collectDividerSegments(node.second, row+firstRows+1, col, secondRows, cols, out)
		return
	}
	firstCols, secondCols, ok := splitSpan(cols)
	if !ok {
		return
	}
	*out = append(*out, dividerSegment{horizontal: false, row: row, col: col + firstCols, length: rows})
	collectDividerSegments(node.first, row, col, rows, firstCols, out)
	collectDividerSegments(node.second, row, col+firstCols+1, rows, secondCols, out)
}

func blankCell() PaneCell {
	return PaneCell{Text: " ", Width: 1}
}

func lineCell(ch string) PaneCell {
	return PaneCell{Text: ch, Width: 1}
}

func activeLineCell(ch string) PaneCell {
	return PaneCell{
		Text:       ch,
		Width:      1,
		HasFgColor: true,
		FgColor:    libghostty.ColorRGB{R: 100, G: 180, B: 255},
		Bold:       true,
	}
}

func dividerGlyph(borders [][]borderState, row, col int) string {
	state := borders[row][col]
	if !state.h && !state.v {
		return " "
	}

	height := len(borders)
	width := 0
	if height > 0 {
		width = len(borders[0])
	}

	left := state.h && col > 0 && borders[row][col-1].h
	right := state.h && col+1 < width && borders[row][col+1].h
	up := state.v && row > 0 && borders[row-1][col].v
	down := state.v && row+1 < height && borders[row+1][col].v

	switch {
	case left && right && up && down:
		return "┼"
	case left && up && down:
		return "┤"
	case right && up && down:
		return "├"
	case left && right && down:
		return "┬"
	case left && right && up:
		return "┴"
	case state.h:
		return "─"
	case state.v:
		return "│"
	default:
		return " "
	}
}

func (w *Window) buildWindowView() WindowView {
	if len(w.layout) == 0 || w.rows <= 0 || w.cols <= 0 {
		return WindowView{}
	}

	screen := make([][]PaneCell, w.rows)
	for r := 0; r < w.rows; r++ {
		screen[r] = make([]PaneCell, w.cols)
		for c := 0; c < w.cols; c++ {
			screen[r][c] = blankCell()
		}
	}

	var (
		activeTitle    string
		activeCursorOn bool
		activeCursorR  = -1
		activeCursorC  = -1
	)

	for _, pl := range w.layout {
		paneRef, ok := w.panes[pl.paneID]
		if !ok {
			continue
		}
		result, _ := askValue(paneRef, GetPaneContent{})
		content, _ := result.(*PaneContent)
		if content == nil {
			continue
		}

		for r := 0; r < pl.rows && pl.row+r < w.rows && r < len(content.Cells); r++ {
			row := content.Cells[r]
			for c := 0; c < pl.cols && pl.col+c < w.cols && c < len(row); c++ {
				screen[pl.row+r][pl.col+c] = row[c]
			}
		}

		if pl.paneID == w.active {
			activeTitle = content.Title
			activeCursorOn = !content.CursorHidden
			if content.CursorRow >= 0 && content.CursorRow < pl.rows {
				activeCursorR = pl.row + content.CursorRow
			}
			if content.CursorCol >= 0 && content.CursorCol < pl.cols {
				activeCursorC = pl.col + content.CursorCol
			}
		}
	}

	borders := make([][]borderState, w.rows)
	for r := 0; r < w.rows; r++ {
		borders[r] = make([]borderState, w.cols)
	}

	segments := make([]dividerSegment, 0)
	collectDividerSegments(w.root, 0, 0, w.rows, w.cols, &segments)
	for _, seg := range segments {
		if seg.horizontal {
			if seg.row < 0 || seg.row >= w.rows {
				continue
			}
			for c := 0; c < seg.length && seg.col+c < w.cols; c++ {
				if seg.col+c >= 0 {
					borders[seg.row][seg.col+c].h = true
				}
			}
			continue
		}
		if seg.col < 0 || seg.col >= w.cols {
			continue
		}
		for r := 0; r < seg.length && seg.row+r < w.rows; r++ {
			if seg.row+r >= 0 {
				borders[seg.row+r][seg.col].v = true
			}
		}
	}

	// Stitch touching split segments into divider cells so nested splits render
	// with connected intersections (e.g. ┼) instead of broken gaps.
	for r := 0; r < w.rows; r++ {
		for c := 0; c < w.cols; c++ {
			if borders[r][c].v {
				if c > 0 && borders[r][c-1].h {
					borders[r][c].h = true
				}
				if c+1 < w.cols && borders[r][c+1].h {
					borders[r][c].h = true
				}
			}
			if borders[r][c].h {
				if r > 0 && borders[r-1][c].v {
					borders[r][c].v = true
				}
				if r+1 < w.rows && borders[r+1][c].v {
					borders[r][c].v = true
				}
			}
		}
	}

	if activeLayout, ok := findLayout(w.layout, w.active); ok {
		for c := 0; c < activeLayout.cols; c++ {
			if row := activeLayout.row - 1; row >= 0 && row < w.rows {
				if col := activeLayout.col + c; col >= 0 && col < w.cols && borders[row][col].h {
					borders[row][col].active = true
				}
			}
			if row := activeLayout.row + activeLayout.rows; row >= 0 && row < w.rows {
				if col := activeLayout.col + c; col >= 0 && col < w.cols && borders[row][col].h {
					borders[row][col].active = true
				}
			}
		}
		for r := 0; r < activeLayout.rows; r++ {
			if col := activeLayout.col - 1; col >= 0 && col < w.cols {
				if row := activeLayout.row + r; row >= 0 && row < w.rows && borders[row][col].v {
					borders[row][col].active = true
				}
			}
			if col := activeLayout.col + activeLayout.cols; col >= 0 && col < w.cols {
				if row := activeLayout.row + r; row >= 0 && row < w.rows && borders[row][col].v {
					borders[row][col].active = true
				}
			}
		}
	}

	for r := 0; r < w.rows; r++ {
		for c := 0; c < w.cols; c++ {
			state := borders[r][c]
			if !state.h && !state.v {
				continue
			}
			glyph := dividerGlyph(borders, r, c)
			if state.active {
				screen[r][c] = activeLineCell(glyph)
			} else {
				screen[r][c] = lineCell(glyph)
			}
		}
	}

	lines := make([]string, w.rows)
	for r := 0; r < w.rows; r++ {
		lines[r] = renderRow(screen[r], w.cols)
	}

	return WindowView{
		Content:   joinLines(lines),
		CursorRow: activeCursorR,
		CursorCol: activeCursorC,
		CursorOn:  activeCursorOn,
		Title:     activeTitle,
	}
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for i := 1; i < len(lines); i++ {
		out += "\n" + lines[i]
	}
	return out
}
