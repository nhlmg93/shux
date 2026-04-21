package shux

import (
	"fmt"
	"math"
)

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
			panic(fmt.Sprintf("window %d: restore layout references missing pane %d", w.id, paneID))
		}
	}
	w.root = restored
	if activePane != 0 {
		if _, ok := w.panes[activePane]; !ok {
			panic(fmt.Sprintf("window %d: restore layout references missing active pane %d", w.id, activePane))
		}
		w.active = activePane
	}
	w.syncLayout()
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
	w.assertInvariants()
}

func (w *Window) switchToPane(index int) {
	if index < 0 || index >= len(w.paneOrder) {
		return
	}
	newActive := w.paneOrder[index]
	if newActive == w.active {
		return
	}

	if _, ok := w.panes[newActive]; !ok {
		panic(fmt.Sprintf("window %d: switchToPane targets missing pane %d", w.id, newActive))
	}
	w.active = newActive
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
	w.assertInvariants()
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
	w.assertInvariants()
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
