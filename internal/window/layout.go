package window

import (
	"fmt"

	"shux/internal/protocol"
)

const minPaneCells = 1
const defaultSplitRatio = uint16(5000)
const splitRatioScale = uint16(10000)

// Rect is pane placement in window cell coordinates, origin top-left.
type Rect struct {
	Col, Row uint16
	Cols     uint16
	Rows     uint16
}

type layoutNode struct {
	PaneID protocol.PaneID
	Split  protocol.SplitDirection
	Ratio  uint16
	First  *layoutNode
	Second *layoutNode
}

func leaf(id protocol.PaneID) *layoutNode {
	return &layoutNode{PaneID: id}
}

func (n *layoutNode) isLeaf() bool {
	return n != nil && n.PaneID.Valid()
}

// Layout is window-local shared tiling geometry. It deliberately does not own
// client focus; clients carry explicit pane targets on commands.
type Layout struct {
	WindowCols, WindowRows uint16
	root                   *layoutNode
	panes                  map[protocol.PaneID]Rect
}

// NewLayout returns an empty layout for a window of the given size.
func NewLayout(windowCols, windowRows uint16) Layout {
	if windowCols == 0 || windowRows == 0 {
		panic(fmt.Sprintf("window: NewLayout: invalid size %dx%d", windowCols, windowRows))
	}
	return Layout{
		WindowCols: windowCols,
		WindowRows: windowRows,
		panes:      make(map[protocol.PaneID]Rect),
	}
}

// SetWindowSize updates the window dimensions and refits existing panes.
// Returns an error if the new size cannot accommodate the current pane tree.
func (l *Layout) SetWindowSize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return fmt.Errorf("window: SetWindowSize: invalid size %dx%d", cols, rows)
	}
	prevCols, prevRows := l.WindowCols, l.WindowRows
	l.WindowCols, l.WindowRows = cols, rows
	if err := l.refit(); err != nil {
		l.WindowCols, l.WindowRows = prevCols, prevRows
		_ = l.refit()
		return err
	}
	return nil
}

// SetSinglePane is the initial layout: one pane fills the window (replaces any prior layout).
func (l *Layout) SetSinglePane(id protocol.PaneID) error {
	if !id.Valid() {
		panic("window: SetSinglePane: invalid PaneID")
	}
	prev := l.root
	l.root = leaf(id)
	if err := l.refit(); err != nil {
		l.root = prev
		_ = l.refit()
		return err
	}
	return nil
}

// CanSplitPane reports whether target is currently present as a split leaf.
func (l *Layout) CanSplitPane(target protocol.PaneID, dir protocol.SplitDirection) error {
	if !target.Valid() {
		return fmt.Errorf("invalid target pane")
	}
	if !dir.Valid() {
		return fmt.Errorf("invalid split direction")
	}
	if l.root == nil {
		return fmt.Errorf("empty layout")
	}
	if !l.hasLeaf(target) {
		return fmt.Errorf("target pane missing")
	}
	r, ok := l.Rect(target)
	if !ok {
		return fmt.Errorf("target pane has no geometry")
	}
	if dir == protocol.SplitVertical && r.Cols < 2*minPaneCells {
		return fmt.Errorf("target pane too narrow")
	}
	if dir == protocol.SplitHorizontal && r.Rows < 2*minPaneCells {
		return fmt.Errorf("target pane too short")
	}
	return nil
}

// SplitPane replaces target leaf with a split branch containing target and newPane.
func (l *Layout) SplitPane(target protocol.PaneID, dir protocol.SplitDirection, newPane protocol.PaneID) error {
	if !newPane.Valid() {
		return fmt.Errorf("invalid new pane")
	}
	if err := l.CanSplitPane(target, dir); err != nil {
		return err
	}
	if l.hasLeaf(newPane) {
		return fmt.Errorf("new pane already exists")
	}
	if !l.splitLeaf(&l.root, target, dir, newPane) {
		return fmt.Errorf("target pane missing")
	}
	return l.refit()
}

// ResizePaneDelta grows/shrinks a pane by moving one edge by Delta cells.
// Positive delta grows toward Edge; negative delta shrinks.
func (l *Layout) ResizePaneDelta(target protocol.PaneID, edge protocol.PaneResizeEdge, delta int) error {
	if !target.Valid() {
		return fmt.Errorf("invalid target pane")
	}
	if !edge.Valid() {
		return fmt.Errorf("invalid pane resize edge")
	}
	if delta == 0 {
		return fmt.Errorf("delta must be non-zero")
	}
	if l.root == nil {
		return fmt.Errorf("empty layout")
	}
	rootRect := Rect{Col: 0, Row: 0, Cols: l.WindowCols, Rows: l.WindowRows}
	path, ok := l.pathToPane(target, l.root, rootRect)
	if !ok {
		return fmt.Errorf("target pane missing")
	}
	for i := len(path) - 1; i >= 0; i-- {
		p := path[i]
		adjust, matched := resizeAdjustForEdge(edge, p.Split, p.inFirst)
		if !matched {
			continue
		}
		total := int(p.axisSize())
		first, _, err := splitRect(p.rect, p.Split, p.Ratio)
		if err != nil {
			continue
		}
		firstSize := int(first.Cols)
		if p.Split == protocol.SplitHorizontal {
			firstSize = int(first.Rows)
		}
		nextFirst := firstSize + adjust*delta
		if nextFirst < minPaneCells || total-nextFirst < minPaneCells {
			continue
		}
		prevRatio := p.Ratio
		p.Ratio = ratioFromFirst(uint16(total), nextFirst)
		if err := l.refit(); err == nil {
			return nil
		}
		p.Ratio = prevRatio
		_ = l.refit()
	}
	return fmt.Errorf("minimum pane size reached")
}

// ApplyPreset rewrites the layout tree for the current panes with a named preset.
func (l *Layout) ApplyPreset(activePaneID protocol.PaneID, preset protocol.LayoutPreset) error {
	if !preset.Valid() {
		return fmt.Errorf("invalid layout preset")
	}
	if !activePaneID.Valid() {
		return fmt.Errorf("invalid active pane")
	}
	ids := l.PaneIDs()
	if len(ids) == 0 {
		return fmt.Errorf("empty layout")
	}
	if !l.hasLeaf(activePaneID) {
		return fmt.Errorf("active pane missing")
	}
	prev := l.root
	switch preset {
	case protocol.LayoutPresetEvenHorizontal:
		l.root = buildEvenTree(ids, protocol.SplitHorizontal)
	case protocol.LayoutPresetEvenVertical:
		l.root = buildEvenTree(ids, protocol.SplitVertical)
	case protocol.LayoutPresetMainHorizontal:
		main := activePaneID
		others := make([]protocol.PaneID, 0, len(ids)-1)
		for _, id := range ids {
			if id != main {
				others = append(others, id)
			}
		}
		l.root = buildMainHorizontalTree(main, others)
	default:
		return fmt.Errorf("invalid layout preset")
	}
	if err := l.refit(); err != nil {
		l.root = prev
		_ = l.refit()
		return err
	}
	return nil
}

// SwapPaneByDirection swaps pane ids in the layout tree with a touching neighbor.
func (l *Layout) SwapPaneByDirection(paneID protocol.PaneID, dir protocol.PaneDirection) (protocol.PaneID, error) {
	if !paneID.Valid() {
		return "", fmt.Errorf("invalid pane")
	}
	if !dir.Valid() {
		return "", fmt.Errorf("invalid direction")
	}
	if l.root == nil || !l.hasLeaf(paneID) {
		return "", fmt.Errorf("pane missing")
	}
	target, ok := l.Rect(paneID)
	if !ok {
		return "", fmt.Errorf("pane has no geometry")
	}
	neighbor, ok := l.findNeighbor(paneID, target, dir)
	if !ok {
		return "", fmt.Errorf("no neighbor in direction")
	}
	if !swapLeafIDs(l.root, paneID, neighbor) {
		return "", fmt.Errorf("pane swap failed")
	}
	if err := l.refit(); err != nil {
		_ = swapLeafIDs(l.root, paneID, neighbor)
		_ = l.refit()
		return "", err
	}
	return neighbor, nil
}

// RemovePane removes a pane leaf from the tree. Errors if the pane isn't present.
func (l *Layout) RemovePane(id protocol.PaneID) error {
	if !id.Valid() {
		return fmt.Errorf("invalid pane")
	}
	if l.root == nil || !l.hasLeaf(id) {
		return fmt.Errorf("pane missing")
	}
	if l.root.isLeaf() {
		if l.root.PaneID != id {
			return fmt.Errorf("pane missing")
		}
		l.root = nil
		return l.refit()
	}
	if !removeLeaf(&l.root, id) {
		return fmt.Errorf("pane missing")
	}
	return l.refit()
}

type layoutPathNode struct {
	*layoutNode
	rect    Rect
	inFirst bool
}

func (l *Layout) pathToPane(target protocol.PaneID, n *layoutNode, r Rect) ([]*layoutPathNode, bool) {
	if n == nil {
		return nil, false
	}
	if n.isLeaf() {
		return nil, n.PaneID == target
	}
	firstRect, secondRect, err := splitRect(r, n.Split, n.Ratio)
	if err != nil {
		return nil, false
	}
	if path, ok := l.pathToPane(target, n.First, firstRect); ok {
		return append(path, &layoutPathNode{layoutNode: n, rect: r, inFirst: true}), true
	}
	if path, ok := l.pathToPane(target, n.Second, secondRect); ok {
		return append(path, &layoutPathNode{layoutNode: n, rect: r, inFirst: false}), true
	}
	return nil, false
}

func (n *layoutPathNode) axisSize() uint16 {
	if n.Split == protocol.SplitVertical {
		return n.rect.Cols
	}
	return n.rect.Rows
}

func resizeAdjustForEdge(edge protocol.PaneResizeEdge, split protocol.SplitDirection, inFirst bool) (int, bool) {
	switch edge {
	case protocol.PaneResizeEdgeLeft:
		return -1, split == protocol.SplitVertical && !inFirst
	case protocol.PaneResizeEdgeRight:
		return 1, split == protocol.SplitVertical && inFirst
	case protocol.PaneResizeEdgeUp:
		return -1, split == protocol.SplitHorizontal && !inFirst
	case protocol.PaneResizeEdgeDown:
		return 1, split == protocol.SplitHorizontal && inFirst
	default:
		return 0, false
	}
}

func ratioFromFirst(total uint16, first int) uint16 {
	if total <= 1 {
		return defaultSplitRatio
	}
	if first < minPaneCells {
		first = minPaneCells
	}
	if first > int(total)-minPaneCells {
		first = int(total) - minPaneCells
	}
	ratio := (first*int(splitRatioScale) + int(total) - 1) / int(total)
	if ratio <= 0 {
		ratio = 1
	}
	if ratio >= int(splitRatioScale) {
		ratio = int(splitRatioScale) - 1
	}
	return uint16(ratio)
}

func removeLeaf(slot **layoutNode, id protocol.PaneID) bool {
	n := *slot
	if n == nil || n.isLeaf() {
		return false
	}
	if n.First != nil && n.First.isLeaf() && n.First.PaneID == id {
		*slot = n.Second
		return true
	}
	if n.Second != nil && n.Second.isLeaf() && n.Second.PaneID == id {
		*slot = n.First
		return true
	}
	return removeLeaf(&n.First, id) || removeLeaf(&n.Second, id)
}

func buildEvenTree(ids []protocol.PaneID, dir protocol.SplitDirection) *layoutNode {
	if len(ids) == 0 {
		return nil
	}
	if len(ids) == 1 {
		return leaf(ids[0])
	}
	mid := len(ids) / 2
	ratio := uint16((uint32(mid) * uint32(splitRatioScale)) / uint32(len(ids)))
	if ratio == 0 || ratio >= splitRatioScale {
		ratio = defaultSplitRatio
	}
	return &layoutNode{
		Split:  dir,
		Ratio:  ratio,
		First:  buildEvenTree(ids[:mid], dir),
		Second: buildEvenTree(ids[mid:], dir),
	}
}

func buildMainHorizontalTree(main protocol.PaneID, others []protocol.PaneID) *layoutNode {
	if len(others) == 0 {
		return leaf(main)
	}
	return &layoutNode{
		Split: protocol.SplitHorizontal,
		Ratio: defaultSplitRatio,
		First: leaf(main),
		Second: func() *layoutNode {
			if len(others) == 1 {
				return leaf(others[0])
			}
			return buildEvenTree(others, protocol.SplitVertical)
		}(),
	}
}

func (l *Layout) splitLeaf(slot **layoutNode, target protocol.PaneID, dir protocol.SplitDirection, newPane protocol.PaneID) bool {
	n := *slot
	if n == nil {
		return false
	}
	if n.isLeaf() {
		if n.PaneID != target {
			return false
		}
		*slot = &layoutNode{
			Split:  dir,
			Ratio:  defaultSplitRatio,
			First:  leaf(target),
			Second: leaf(newPane),
		}
		return true
	}
	return l.splitLeaf(&n.First, target, dir, newPane) || l.splitLeaf(&n.Second, target, dir, newPane)
}

func swapLeafIDs(n *layoutNode, left protocol.PaneID, right protocol.PaneID) bool {
	if n == nil {
		return false
	}
	var leftFound, rightFound *layoutNode
	var walk func(node *layoutNode)
	walk = func(node *layoutNode) {
		if node == nil || (!node.isLeaf() && leftFound != nil && rightFound != nil) {
			return
		}
		if node.isLeaf() {
			switch node.PaneID {
			case left:
				leftFound = node
			case right:
				rightFound = node
			}
			return
		}
		walk(node.First)
		walk(node.Second)
	}
	walk(n)
	if leftFound == nil || rightFound == nil {
		return false
	}
	leftFound.PaneID, rightFound.PaneID = rightFound.PaneID, leftFound.PaneID
	return true
}

func (l *Layout) hasLeaf(target protocol.PaneID) bool {
	return hasLeaf(l.root, target)
}

func hasLeaf(n *layoutNode, target protocol.PaneID) bool {
	if n == nil {
		return false
	}
	if n.isLeaf() {
		return n.PaneID == target
	}
	return hasLeaf(n.First, target) || hasLeaf(n.Second, target)
}

func (l *Layout) refit() error {
	l.panes = make(map[protocol.PaneID]Rect)
	if l.root == nil {
		return nil
	}
	root := Rect{Col: 0, Row: 0, Cols: l.WindowCols, Rows: l.WindowRows}
	if err := l.assertRectInWindow(root); err != nil {
		return err
	}
	return l.fitNode(l.root, root)
}

func (l *Layout) fitNode(n *layoutNode, r Rect) error {
	if n == nil {
		return nil
	}
	if n.isLeaf() {
		if err := l.assertRectInWindow(r); err != nil {
			return err
		}
		l.panes[n.PaneID] = r
		return nil
	}
	first, second, err := splitRect(r, n.Split, n.Ratio)
	if err != nil {
		return err
	}
	if err := l.fitNode(n.First, first); err != nil {
		return err
	}
	return l.fitNode(n.Second, second)
}

func splitRect(r Rect, dir protocol.SplitDirection, ratio uint16) (Rect, Rect, error) {
	if ratio == 0 || ratio >= splitRatioScale {
		ratio = defaultSplitRatio
	}
	if dir == protocol.SplitVertical {
		if r.Cols < 2*minPaneCells {
			return Rect{}, Rect{}, fmt.Errorf("window: layout: pane too narrow for split")
		}
		firstCols := scalePortion(r.Cols, ratio)
		secondCols := r.Cols - firstCols
		return Rect{Col: r.Col, Row: r.Row, Cols: firstCols, Rows: r.Rows},
			Rect{Col: r.Col + firstCols, Row: r.Row, Cols: secondCols, Rows: r.Rows},
			nil
	}
	if r.Rows < 2*minPaneCells {
		return Rect{}, Rect{}, fmt.Errorf("window: layout: pane too short for split")
	}
	firstRows := scalePortion(r.Rows, ratio)
	secondRows := r.Rows - firstRows
	return Rect{Col: r.Col, Row: r.Row, Cols: r.Cols, Rows: firstRows},
		Rect{Col: r.Col, Row: r.Row + firstRows, Cols: r.Cols, Rows: secondRows},
		nil
}

func scalePortion(total uint16, ratio uint16) uint16 {
	n := int(uint32(total) * uint32(ratio) / uint32(splitRatioScale))
	if n < minPaneCells {
		n = minPaneCells
	}
	if n >= int(total) {
		n = int(total) - minPaneCells
	}
	return uint16(n)
}

// Rect returns a copy of the pane's rectangle, or false if the pane is unknown.
func (l *Layout) Rect(id protocol.PaneID) (Rect, bool) {
	if l.panes == nil {
		return Rect{}, false
	}
	r, ok := l.panes[id]
	return r, ok
}

// PaneIDs returns pane ids in stable render order.
func (l *Layout) PaneIDs() []protocol.PaneID {
	ids := make([]protocol.PaneID, 0, len(l.panes))
	collectPaneIDs(l.root, &ids)
	return ids
}

func collectPaneIDs(n *layoutNode, ids *[]protocol.PaneID) {
	if n == nil {
		return
	}
	if n.isLeaf() {
		*ids = append(*ids, n.PaneID)
		return
	}
	collectPaneIDs(n.First, ids)
	collectPaneIDs(n.Second, ids)
}

func (l *Layout) findNeighbor(paneID protocol.PaneID, src Rect, dir protocol.PaneDirection) (protocol.PaneID, bool) {
	bestID := protocol.PaneID("")
	bestOverlap := -1
	bestOffset := int(^uint(0) >> 1)
	for _, candidateID := range l.PaneIDs() {
		if candidateID == paneID {
			continue
		}
		candidate, ok := l.Rect(candidateID)
		if !ok {
			continue
		}
		touching, overlap, offset := touchingScore(src, candidate, dir)
		if !touching {
			continue
		}
		if overlap > bestOverlap || (overlap == bestOverlap && offset < bestOffset) {
			bestID = candidateID
			bestOverlap = overlap
			bestOffset = offset
		}
	}
	if !bestID.Valid() {
		return "", false
	}
	return bestID, true
}

func touchingScore(src Rect, other Rect, dir protocol.PaneDirection) (bool, int, int) {
	srcLeft := int(src.Col)
	srcRight := int(src.Col) + int(src.Cols)
	srcTop := int(src.Row)
	srcBottom := int(src.Row) + int(src.Rows)
	otherLeft := int(other.Col)
	otherRight := int(other.Col) + int(other.Cols)
	otherTop := int(other.Row)
	otherBottom := int(other.Row) + int(other.Rows)

	switch dir {
	case protocol.PaneDirectionLeft:
		if otherRight != srcLeft {
			return false, 0, 0
		}
		overlap := rangeOverlap(srcTop, srcBottom, otherTop, otherBottom)
		if overlap <= 0 {
			return false, 0, 0
		}
		return true, overlap, abs((srcTop + srcBottom) - (otherTop + otherBottom))
	case protocol.PaneDirectionRight:
		if otherLeft != srcRight {
			return false, 0, 0
		}
		overlap := rangeOverlap(srcTop, srcBottom, otherTop, otherBottom)
		if overlap <= 0 {
			return false, 0, 0
		}
		return true, overlap, abs((srcTop + srcBottom) - (otherTop + otherBottom))
	case protocol.PaneDirectionUp:
		if otherBottom != srcTop {
			return false, 0, 0
		}
		overlap := rangeOverlap(srcLeft, srcRight, otherLeft, otherRight)
		if overlap <= 0 {
			return false, 0, 0
		}
		return true, overlap, abs((srcLeft + srcRight) - (otherLeft + otherRight))
	case protocol.PaneDirectionDown:
		if otherTop != srcBottom {
			return false, 0, 0
		}
		overlap := rangeOverlap(srcLeft, srcRight, otherLeft, otherRight)
		if overlap <= 0 {
			return false, 0, 0
		}
		return true, overlap, abs((srcLeft + srcRight) - (otherLeft + otherRight))
	default:
		return false, 0, 0
	}
}

func rangeOverlap(aStart, aEnd, bStart, bEnd int) int {
	start := aStart
	if bStart > start {
		start = bStart
	}
	end := aEnd
	if bEnd < end {
		end = bEnd
	}
	return end - start
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (l *Layout) assertRectInWindow(r Rect) error {
	if r.Cols == 0 || r.Rows == 0 {
		return fmt.Errorf("window: layout: zero pane size")
	}
	endCol := int(r.Col) + int(r.Cols)
	endRow := int(r.Row) + int(r.Rows)
	if endCol > int(l.WindowCols) || endRow > int(l.WindowRows) {
		return fmt.Errorf("window: layout: pane out of bounds")
	}
	return nil
}
