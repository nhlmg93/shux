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

// SetWindowSize updates the window dimensions and refits existing panes without removing them.
func (l *Layout) SetWindowSize(cols, rows uint16) {
	if cols == 0 || rows == 0 {
		panic(fmt.Sprintf("window: SetWindowSize: invalid size %dx%d", cols, rows))
	}
	l.WindowCols, l.WindowRows = cols, rows
	l.refit()
}

// SetSinglePane is the initial layout: one pane fills the window (replaces any prior layout).
func (l *Layout) SetSinglePane(id protocol.PaneID) {
	if !id.Valid() {
		panic("window: SetSinglePane: invalid PaneID")
	}
	l.root = leaf(id)
	l.refit()
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
	l.refit()
	return nil
}

// SplitActive is retained only for old call sites; it splits the first pane in stable order.
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
		l.refit()
		return nil
	}
	if !removeLeaf(&l.root, id) {
		return fmt.Errorf("pane missing")
	}
	l.refit()
	return nil
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

func (l *Layout) SplitActive(dir protocol.SplitDirection, newPane protocol.PaneID) {
	ids := l.PaneIDs()
	if len(ids) == 0 {
		panic("window: SplitActive: empty layout")
	}
	if err := l.SplitPane(ids[0], dir, newPane); err != nil {
		panic("window: SplitActive: " + err.Error())
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

func (l *Layout) refit() {
	l.panes = make(map[protocol.PaneID]Rect)
	if l.root == nil {
		return
	}
	root := Rect{Col: 0, Row: 0, Cols: l.WindowCols, Rows: l.WindowRows}
	if err := l.assertRectInWindow(root); err != nil {
		panic(err)
	}
	l.fitNode(l.root, root)
}

func (l *Layout) fitNode(n *layoutNode, r Rect) {
	if n == nil {
		return
	}
	if n.isLeaf() {
		if err := l.assertRectInWindow(r); err != nil {
			panic(err)
		}
		l.panes[n.PaneID] = r
		return
	}
	first, second := splitRect(r, n.Split, n.Ratio)
	l.fitNode(n.First, first)
	l.fitNode(n.Second, second)
}

func splitRect(r Rect, dir protocol.SplitDirection, ratio uint16) (Rect, Rect) {
	if ratio == 0 || ratio >= splitRatioScale {
		ratio = defaultSplitRatio
	}
	if dir == protocol.SplitVertical {
		if r.Cols < 2*minPaneCells {
			panic("window: layout: pane too narrow for split")
		}
		firstCols := scalePortion(r.Cols, ratio)
		secondCols := r.Cols - firstCols
		return Rect{Col: r.Col, Row: r.Row, Cols: firstCols, Rows: r.Rows},
			Rect{Col: r.Col + firstCols, Row: r.Row, Cols: secondCols, Rows: r.Rows}
	}
	if r.Rows < 2*minPaneCells {
		panic("window: layout: pane too short for split")
	}
	firstRows := scalePortion(r.Rows, ratio)
	secondRows := r.Rows - firstRows
	return Rect{Col: r.Col, Row: r.Row, Cols: r.Cols, Rows: firstRows},
		Rect{Col: r.Col, Row: r.Row + firstRows, Cols: r.Cols, Rows: secondRows}
}

func scalePortion(total uint16, ratio uint16) uint16 {
	if total < 2*minPaneCells {
		panic("window: layout: split total too small")
	}
	n := int(uint32(total) * uint32(ratio) / uint32(splitRatioScale))
	if n < minPaneCells {
		n = minPaneCells
	}
	if n >= int(total) {
		n = int(total) - minPaneCells
	}
	return uint16(n)
}

// Rect returns a copy of the pane’s rectangle, or false if the pane is unknown.
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
