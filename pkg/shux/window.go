package shux

import (
	"fmt"
	"math"

	"github.com/mitchellh/go-libghostty"
)

// WindowRef is a reference to a window loop. Methods are promoted from loopRef.
type WindowRef struct {
	*loopRef
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
	ratio  float64
	first  *splitNode
	second *splitNode
}

const defaultSplitRatio = 0.5

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

type dividerHit struct {
	node       *splitNode
	horizontal bool
	row        int
	col        int
	length     int
	rectRow    int
	rectCol    int
	rectRows   int
	rectCols   int
}

func (h dividerHit) contains(row, col int) bool {
	if h.horizontal {
		return row == h.row && col >= h.col && col < h.col+h.length
	}
	return col == h.col && row >= h.row && row < h.row+h.length
}

type borderState struct {
	h      bool
	v      bool
	active bool
}

type renderedActivePane struct {
	title    string
	cursorOn bool
	cursorR  int
	cursorC  int
}

func newRenderedActivePane() renderedActivePane {
	return renderedActivePane{cursorR: -1, cursorC: -1}
}

func (p renderedActivePane) toWindowView(content string) WindowView {
	return WindowView{
		Content:   content,
		CursorRow: p.cursorR,
		CursorCol: p.cursorC,
		CursorOn:  p.cursorOn,
		Title:     p.title,
	}
}

type Window struct {
	ref       *WindowRef
	logger    ShuxLogger
	parent    *SessionRef
	id        uint32
	panes     map[uint32]*PaneRef
	paneOrder OrderedIDList
	active    uint32
	paneID    uint32

	root             *splitNode
	layout           []paneLayout
	splitDir         SplitDir
	rows             int
	cols             int
	dividerDrag      *dividerHit
	mouseCapturePane uint32
}

func NewWindow(id uint32) *Window {
	return &Window{
		id:    id,
		panes: make(map[uint32]*PaneRef),
	}
}

func StartWindow(id uint32, parent *SessionRef, logger ShuxLogger) *WindowRef {
	w := NewWindow(id)
	w.parent = parent
	w.logger = logger
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
		w.logger.Errorf("window: crash id=%d reason=%v", w.id, reason)
		return
	}
	w.logger.Infof("window: terminate id=%d", w.id)
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
	case ResizePane:
		w.resizePane(m.Dir, m.Amount)
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
	case MouseInput:
		w.handleMouseInput(m)
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
		PaneOrder:  w.paneOrder.Clone(),
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

	ref := StartPane(paneID, cmd.Rows, cmd.Cols, cmd.Shell, cmd.CWD, w.ref, w.logger)
	w.panes[paneID] = ref
	w.paneOrder.Add(paneID)

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
	newRef := StartPane(newPaneID, w.rows, w.cols, shell, cwd, w.ref, w.logger)
	w.panes[newPaneID] = newRef
	w.paneOrder.Add(newPaneID)
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
			ratio:  defaultSplitRatio,
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
		Ratio:  node.ratio,
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
	ratio := node.Ratio
	if ratio <= 0 || ratio >= 1 {
		ratio = defaultSplitRatio
	}
	return &splitNode{
		dir:    node.Dir,
		ratio:  ratio,
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
			w.logger.Warnf("window %d: restore layout references missing pane %d", w.id, paneID)
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

func (w *Window) resizePane(dir PaneNavDir, amount int) {
	if w.root == nil || w.active == 0 || amount <= 0 {
		return
	}

	path := make([]splitPathStep, 0, 8)
	if !collectSplitPath(w.root, w.active, 0, 0, w.rows, w.cols, &path) {
		return
	}

	for _, step := range path {
		if !step.matches(dir) {
			continue
		}
		if adjustSplitRatioByDelta(step.node, step.span(), step.delta(dir, amount)) {
			w.syncLayout()
			if w.parent != nil {
				w.parent.Send(PaneContentUpdated{})
			}
		}
		return
	}
}

func (w *Window) handleMouseInput(input MouseInput) {
	if w.root == nil || w.rows <= 0 || w.cols <= 0 {
		return
	}

	if w.dividerDrag != nil {
		if input.Action == MouseActionMotion || input.Action == MouseActionRelease {
			if setSplitRatioFromHit(*w.dividerDrag, input.Row, input.Col) {
				w.syncLayout()
				if w.parent != nil {
					w.parent.Send(PaneContentUpdated{})
				}
			}
			if input.Action == MouseActionRelease {
				w.dividerDrag = nil
			}
			return
		}
	}

	if input.Action == MouseActionPress && input.Button == MouseButtonLeft {
		if hit, ok := w.dividerHitAt(input.Row, input.Col); ok {
			copied := hit
			w.dividerDrag = &copied
			setSplitRatioFromHit(hit, input.Row, input.Col)
			w.syncLayout()
			if w.parent != nil {
				w.parent.Send(PaneContentUpdated{})
			}
			return
		}
	}

	targetID := w.targetPaneForMouse(input)
	if targetID == 0 {
		return
	}
	if input.Action == MouseActionPress {
		w.mouseCapturePane = targetID
	}
	if input.Action == MouseActionRelease {
		defer func() { w.mouseCapturePane = 0 }()
	}
	w.sendMouseToPane(targetID, input)
}

type splitPathStep struct {
	node   *splitNode
	branch int
	rows   int
	cols   int
}

func (s splitPathStep) span() int {
	if s.node == nil {
		return 0
	}
	if s.node.dir == SplitH {
		return s.rows
	}
	return s.cols
}

func (s splitPathStep) matches(dir PaneNavDir) bool {
	if s.node == nil {
		return false
	}
	switch dir {
	case PaneNavLeft:
		return s.node.dir == SplitV && s.branch == 1
	case PaneNavRight:
		return s.node.dir == SplitV && s.branch == 0
	case PaneNavUp:
		return s.node.dir == SplitH && s.branch == 1
	case PaneNavDown:
		return s.node.dir == SplitH && s.branch == 0
	default:
		return false
	}
}

func (s splitPathStep) delta(dir PaneNavDir, amount int) float64 {
	if amount <= 0 {
		amount = 1
	}
	avail := s.span() - 1
	if avail <= 0 {
		return 0
	}
	delta := float64(amount) / float64(avail)
	switch dir {
	case PaneNavLeft, PaneNavUp:
		return -delta
	case PaneNavRight, PaneNavDown:
		return delta
	default:
		return 0
	}
}

func collectSplitPath(node *splitNode, target uint32, row, col, rows, cols int, out *[]splitPathStep) bool {
	if node == nil {
		return false
	}
	if node.isLeaf() {
		return node.paneID == target
	}
	if node.dir == SplitH {
		firstRows, secondRows, ok := splitSpanWithRatio(rows, node.ratio)
		if !ok {
			return collectSplitPath(node.first, target, row, col, rows, cols, out)
		}
		if collectSplitPath(node.first, target, row, col, firstRows, cols, out) {
			*out = append(*out, splitPathStep{node: node, branch: 0, rows: rows, cols: cols})
			return true
		}
		if collectSplitPath(node.second, target, row+firstRows+1, col, secondRows, cols, out) {
			*out = append(*out, splitPathStep{node: node, branch: 1, rows: rows, cols: cols})
			return true
		}
		return false
	}

	firstCols, secondCols, ok := splitSpanWithRatio(cols, node.ratio)
	if !ok {
		return collectSplitPath(node.first, target, row, col, rows, cols, out)
	}
	if collectSplitPath(node.first, target, row, col, rows, firstCols, out) {
		*out = append(*out, splitPathStep{node: node, branch: 0, rows: rows, cols: cols})
		return true
	}
	if collectSplitPath(node.second, target, row, col+firstCols+1, rows, secondCols, out) {
		*out = append(*out, splitPathStep{node: node, branch: 1, rows: rows, cols: cols})
		return true
	}
	return false
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

func (w *Window) targetPaneForMouse(input MouseInput) uint32 {
	if w.mouseCapturePane != 0 && input.Action != MouseActionPress {
		if _, ok := w.panes[w.mouseCapturePane]; ok {
			return w.mouseCapturePane
		}
	}
	if layout, ok := w.paneAt(input.Row, input.Col); ok {
		if input.Action == MouseActionPress && layout.paneID != w.active {
			w.active = layout.paneID
			if w.parent != nil {
				w.parent.Send(PaneContentUpdated{})
			}
		}
		return layout.paneID
	}
	return 0
}

func (w *Window) sendMouseToPane(paneID uint32, input MouseInput) {
	layout, ok := findLayout(w.layout, paneID)
	if !ok {
		return
	}
	paneRef, ok := w.panes[paneID]
	if !ok {
		return
	}
	local := input
	local.Row -= layout.row
	local.Col -= layout.col
	if local.Row < 0 {
		local.Row = 0
	}
	if local.Col < 0 {
		local.Col = 0
	}
	if local.Row >= layout.rows {
		local.Row = layout.rows - 1
	}
	if local.Col >= layout.cols {
		local.Col = layout.cols - 1
	}
	paneRef.Send(local)
}

func (w *Window) paneAt(row, col int) (paneLayout, bool) {
	for _, pl := range w.layout {
		if row >= pl.row && row < pl.row+pl.rows && col >= pl.col && col < pl.col+pl.cols {
			return pl, true
		}
	}
	return paneLayout{}, false
}

func (w *Window) dividerHitAt(row, col int) (dividerHit, bool) {
	hits := make([]dividerHit, 0, len(w.paneOrder))
	collectDividerHits(w.root, 0, 0, w.rows, w.cols, &hits)
	for _, hit := range hits {
		if hit.contains(row, col) {
			return hit, true
		}
	}
	return dividerHit{}, false
}

func collectDividerHits(node *splitNode, row, col, rows, cols int, out *[]dividerHit) {
	if node == nil || node.isLeaf() {
		return
	}
	if node.dir == SplitH {
		firstRows, secondRows, ok := splitSpanWithRatio(rows, node.ratio)
		if !ok {
			return
		}
		*out = append(*out, dividerHit{node: node, horizontal: true, row: row + firstRows, col: col, length: cols, rectRow: row, rectCol: col, rectRows: rows, rectCols: cols})
		collectDividerHits(node.first, row, col, firstRows, cols, out)
		collectDividerHits(node.second, row+firstRows+1, col, secondRows, cols, out)
		return
	}
	firstCols, secondCols, ok := splitSpanWithRatio(cols, node.ratio)
	if !ok {
		return
	}
	*out = append(*out, dividerHit{node: node, horizontal: false, row: row, col: col + firstCols, length: rows, rectRow: row, rectCol: col, rectRows: rows, rectCols: cols})
	collectDividerHits(node.first, row, col, rows, firstCols, out)
	collectDividerHits(node.second, row, col+firstCols+1, rows, secondCols, out)
}

func setSplitRatioFromHit(hit dividerHit, row, col int) bool {
	if hit.node == nil {
		return false
	}
	if hit.horizontal {
		avail := hit.rectRows - 1
		if avail <= 0 {
			return false
		}
		first := row - hit.rectRow
		return setSplitRatio(hit.node, float64(first)/float64(avail), avail)
	}
	avail := hit.rectCols - 1
	if avail <= 0 {
		return false
	}
	first := col - hit.rectCol
	return setSplitRatio(hit.node, float64(first)/float64(avail), avail)
}

func setSplitRatio(node *splitNode, ratio float64, avail int) bool {
	if node == nil {
		return false
	}
	ratio = clampSplitRatio(ratio, avail)
	if math.Abs(node.ratio-ratio) < 0.0001 {
		return false
	}
	node.ratio = ratio
	return true
}

func adjustSplitRatioByDelta(node *splitNode, span int, delta float64) bool {
	if node == nil || delta == 0 {
		return false
	}
	avail := span - 1
	if avail <= 0 {
		return false
	}
	return setSplitRatio(node, node.ratio+delta, avail)
}

func clampSplitRatio(ratio float64, avail int) float64 {
	if avail <= 1 {
		if ratio < 0 {
			return 0
		}
		if ratio > 1 {
			return 1
		}
		if ratio == 0 {
			return 1
		}
		return ratio
	}
	minRatio := 1.0 / float64(avail)
	maxRatio := float64(avail-1) / float64(avail)
	if ratio < minRatio {
		return minRatio
	}
	if ratio > maxRatio {
		return maxRatio
	}
	if ratio <= 0 || ratio >= 1 {
		return defaultSplitRatio
	}
	return ratio
}

func (w *Window) resizeAllPanes(rows, cols int) {
	w.logger.Infof("window %d: resizing to %dx%d (was %dx%d)", w.id, rows, cols, w.rows, w.cols)
	w.rows = rows
	w.cols = cols
	w.syncLayout()
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
}

func (w *Window) handlePaneExited(id uint32) {
	currentIdx := w.paneOrder.IndexOf(w.active)
	if w.mouseCapturePane == id {
		w.mouseCapturePane = 0
	}
	w.dividerDrag = nil
	delete(w.panes, id)
	w.paneOrder.Remove(id)

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
			w.logger.Infof("window %d: resizing pane %d to %dx%d at %d,%d", w.id, pl.paneID, pl.rows, pl.cols, pl.row, pl.col)
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
		firstRows, secondRows, ok := splitSpanWithRatio(rows, node.ratio)
		if !ok {
			layoutSplitTree(node.first, row, col, rows, cols, out)
			return
		}
		layoutSplitTree(node.first, row, col, firstRows, cols, out)
		layoutSplitTree(node.second, row+firstRows+1, col, secondRows, cols, out)
		return
	}
	firstCols, secondCols, ok := splitSpanWithRatio(cols, node.ratio)
	if !ok {
		layoutSplitTree(node.first, row, col, rows, cols, out)
		return
	}
	layoutSplitTree(node.first, row, col, rows, firstCols, out)
	layoutSplitTree(node.second, row, col+firstCols+1, rows, secondCols, out)
}

func splitSpanWithRatio(total int, ratio float64) (first, second int, ok bool) {
	if total <= 1 {
		return total, 0, false
	}
	avail := total - 1
	if avail <= 1 {
		return 1, avail - 1, true
	}
	first = int(math.Round(float64(avail) * clampSplitRatio(ratio, avail)))
	if first < 1 {
		first = 1
	}
	if first > avail-1 {
		first = avail - 1
	}
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
		firstRows, secondRows, ok := splitSpanWithRatio(rows, node.ratio)
		if !ok {
			return
		}
		*out = append(*out, dividerSegment{horizontal: true, row: row + firstRows, col: col, length: cols})
		collectDividerSegments(node.first, row, col, firstRows, cols, out)
		collectDividerSegments(node.second, row+firstRows+1, col, secondRows, cols, out)
		return
	}
	firstCols, secondCols, ok := splitSpanWithRatio(cols, node.ratio)
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

func newBlankScreen(rows, cols int) [][]PaneCell {
	screen := make([][]PaneCell, rows)
	for r := 0; r < rows; r++ {
		screen[r] = make([]PaneCell, cols)
		for c := 0; c < cols; c++ {
			screen[r][c] = blankCell()
		}
	}
	return screen
}

func renderPanesToScreen(screen [][]PaneCell, layout []paneLayout, panes map[uint32]*PaneRef, activePaneID uint32) renderedActivePane {
	activePane := newRenderedActivePane()
	if len(screen) == 0 {
		return activePane
	}
	rows := len(screen)
	cols := len(screen[0])

	for _, pl := range layout {
		paneRef, ok := panes[pl.paneID]
		if !ok {
			continue
		}
		result, _ := askValue(paneRef, GetPaneContent{})
		content, _ := result.(*PaneContent)
		if content == nil {
			continue
		}

		for r := 0; r < pl.rows && pl.row+r < rows && r < len(content.Cells); r++ {
			row := content.Cells[r]
			for c := 0; c < pl.cols && pl.col+c < cols && c < len(row); c++ {
				screen[pl.row+r][pl.col+c] = row[c]
			}
		}

		if pl.paneID != activePaneID {
			continue
		}
		activePane.title = content.Title
		activePane.cursorOn = !content.CursorHidden
		if content.CursorRow >= 0 && content.CursorRow < pl.rows {
			activePane.cursorR = pl.row + content.CursorRow
		}
		if content.CursorCol >= 0 && content.CursorCol < pl.cols {
			activePane.cursorC = pl.col + content.CursorCol
		}
	}

	return activePane
}

func newBorderGrid(rows, cols int) [][]borderState {
	borders := make([][]borderState, rows)
	for r := 0; r < rows; r++ {
		borders[r] = make([]borderState, cols)
	}
	return borders
}

func collectDividerSegmentsList(root *splitNode, rows, cols int) []dividerSegment {
	segments := make([]dividerSegment, 0)
	collectDividerSegments(root, 0, 0, rows, cols, &segments)
	return segments
}

func applyDividerSegment(borders [][]borderState, seg dividerSegment) {
	if len(borders) == 0 {
		return
	}
	rows := len(borders)
	cols := len(borders[0])

	if seg.horizontal {
		if seg.row < 0 || seg.row >= rows {
			return
		}
		for c := 0; c < seg.length && seg.col+c < cols; c++ {
			if seg.col+c >= 0 {
				borders[seg.row][seg.col+c].h = true
			}
		}
		return
	}

	if seg.col < 0 || seg.col >= cols {
		return
	}
	for r := 0; r < seg.length && seg.row+r < rows; r++ {
		if seg.row+r >= 0 {
			borders[seg.row+r][seg.col].v = true
		}
	}
}

func buildDividerBorders(root *splitNode, rows, cols int) [][]borderState {
	borders := newBorderGrid(rows, cols)
	for _, seg := range collectDividerSegmentsList(root, rows, cols) {
		applyDividerSegment(borders, seg)
	}
	stitchDividerBorders(borders)
	return borders
}

func stitchDividerBorders(borders [][]borderState) {
	rows := len(borders)
	if rows == 0 {
		return
	}
	cols := len(borders[0])

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if borders[r][c].v {
				if c > 0 && borders[r][c-1].h {
					borders[r][c].h = true
				}
				if c+1 < cols && borders[r][c+1].h {
					borders[r][c].h = true
				}
			}
			if borders[r][c].h {
				if r > 0 && borders[r-1][c].v {
					borders[r][c].v = true
				}
				if r+1 < rows && borders[r+1][c].v {
					borders[r][c].v = true
				}
			}
		}
	}
}

func markActiveHorizontalBorder(borders [][]borderState, row, startCol, length int) {
	if row < 0 || row >= len(borders) {
		return
	}
	for offset := 0; offset < length; offset++ {
		col := startCol + offset
		if col < 0 || col >= len(borders[row]) || !borders[row][col].h {
			continue
		}
		borders[row][col].active = true
	}
}

func markActiveVerticalBorder(borders [][]borderState, startRow, col, length int) {
	if len(borders) == 0 {
		return
	}
	for offset := 0; offset < length; offset++ {
		row := startRow + offset
		if row < 0 || row >= len(borders) {
			continue
		}
		if col < 0 || col >= len(borders[row]) || !borders[row][col].v {
			continue
		}
		borders[row][col].active = true
	}
}

func markActivePaneBorders(borders [][]borderState, layout []paneLayout, activePaneID uint32) {
	activeLayout, ok := findLayout(layout, activePaneID)
	if !ok {
		return
	}
	markActiveHorizontalBorder(borders, activeLayout.row-1, activeLayout.col, activeLayout.cols)
	markActiveHorizontalBorder(borders, activeLayout.row+activeLayout.rows, activeLayout.col, activeLayout.cols)
	markActiveVerticalBorder(borders, activeLayout.row, activeLayout.col-1, activeLayout.rows)
	markActiveVerticalBorder(borders, activeLayout.row, activeLayout.col+activeLayout.cols, activeLayout.rows)
}

func drawDividerBorders(screen [][]PaneCell, borders [][]borderState) {
	for r := range borders {
		for c := range borders[r] {
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
}

func renderScreenLines(screen [][]PaneCell, cols int) []string {
	lines := make([]string, len(screen))
	for r := range screen {
		lines[r] = renderRow(screen[r], cols)
	}
	return lines
}

func (w *Window) buildWindowView() WindowView {
	if len(w.layout) == 0 || w.rows <= 0 || w.cols <= 0 {
		return WindowView{}
	}

	screen := newBlankScreen(w.rows, w.cols)
	activePane := renderPanesToScreen(screen, w.layout, w.panes, w.active)
	borders := buildDividerBorders(w.root, w.rows, w.cols)
	markActivePaneBorders(borders, w.layout, w.active)
	drawDividerBorders(screen, borders)

	return activePane.toWindowView(joinLines(renderScreenLines(screen, w.cols)))
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
