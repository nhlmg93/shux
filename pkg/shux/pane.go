package shux

import (
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mitchellh/go-libghostty"
	"github.com/nhlmg93/gotor/actor"
)

var _ Resizable = (*Pane)(nil)

type PaneCell struct {
	Text         string
	Width        int
	FgColor      libghostty.ColorRGB
	BgColor      libghostty.ColorRGB
	Bold         bool
	Italic       bool
	Underline    bool
	Blink        bool
	Reverse      bool
	HasHyperlink bool
}

type PaneContent struct {
	Lines          []string
	Cells          [][]PaneCell
	CursorRow      int
	CursorCol      int
	InAltScreen    bool
	CursorHidden   bool
	Title          string
	BellCount      uint64
	ScrollbackRows uint
}

type Pane struct {
	id            uint32
	self          *actor.Ref
	term          *libghostty.Terminal
	renderState   *libghostty.RenderState
	rowIterator   *libghostty.RenderStateRowIterator
	rowCells      *libghostty.RenderStateRowCells
	keyEncoder    *libghostty.KeyEncoder
	pty           *PTY
	rows          int
	cols          int
	shell         string
	windowTitle   string
	bellCount     uint64
	dirty         bool
	cachedContent *PaneContent
	updateTimer   *actor.Timer
	updatePending bool
	stopped       bool
}

type panePTYData struct{ Data []byte }
type paneFlushUpdate struct{}
type paneProcessExited struct{ Err error }

func NewPane(id uint32, rows, cols int, shell string) *Pane {
	return &Pane{
		id:    id,
		rows:  rows,
		cols:  cols,
		shell: shell,
	}
}

func (p *Pane) Init() error {
	Infof("pane %d: initializing", p.id)
	p.self = actor.Self()

	ghosttyTerm, err := libghostty.NewTerminal(
		libghostty.WithSize(uint16(p.cols), uint16(p.rows)),
		libghostty.WithMaxScrollback(10000),
		libghostty.WithTitleChanged(func(t *libghostty.Terminal) {
			if title, err := t.Title(); err == nil {
				p.windowTitle = title
			}
			p.markDirty()
		}),
		libghostty.WithBell(func(t *libghostty.Terminal) {
			p.bellCount++
			p.markDirty()
		}),
		libghostty.WithWritePty(func(t *libghostty.Terminal, data []byte) {
			if p.pty != nil && p.pty.TTY != nil {
				_, _ = p.pty.TTY.Write(data)
			}
		}),
	)
	if err != nil {
		return err
	}

	renderState, err := libghostty.NewRenderState()
	if err != nil {
		ghosttyTerm.Close()
		return err
	}

	rowIterator, err := libghostty.NewRenderStateRowIterator()
	if err != nil {
		renderState.Close()
		ghosttyTerm.Close()
		return err
	}

	rowCells, err := libghostty.NewRenderStateRowCells()
	if err != nil {
		rowIterator.Close()
		renderState.Close()
		ghosttyTerm.Close()
		return err
	}

	keyEncoder, err := libghostty.NewKeyEncoder()
	if err != nil {
		rowCells.Close()
		rowIterator.Close()
		renderState.Close()
		ghosttyTerm.Close()
		return err
	}

	Infof("pane %d: starting shell %s", p.id, p.shell)
	cmd := exec.Command(p.shell)
	env := os.Environ()
	termSet := false
	for _, e := range env {
		if len(e) > 5 && e[:5] == "TERM=" {
			termSet = true
			break
		}
	}
	if !termSet {
		env = append(env, "TERM=xterm-256color")
	}
	cmd.Env = env

	pty, err := StartWithSize(cmd, p.rows, p.cols)
	if err != nil {
		keyEncoder.Close()
		rowCells.Close()
		rowIterator.Close()
		renderState.Close()
		ghosttyTerm.Close()
		return err
	}

	p.term = ghosttyTerm
	p.renderState = renderState
	p.rowIterator = rowIterator
	p.rowCells = rowCells
	p.keyEncoder = keyEncoder
	p.pty = pty
	p.dirty = true

	go p.readLoop()
	p.scheduleContentUpdate(0)
	return nil
}

func (p *Pane) Terminate(reason error) {
	Infof("pane %d: terminating (%v)", p.id, reason)
	if p.updateTimer != nil {
		p.updateTimer.Stop()
	}
	if p.pty != nil {
		_ = p.pty.Close()
	}
	if p.keyEncoder != nil {
		p.keyEncoder.Close()
	}
	if p.rowCells != nil {
		p.rowCells.Close()
	}
	if p.rowIterator != nil {
		p.rowIterator.Close()
	}
	if p.renderState != nil {
		p.renderState.Close()
	}
	if p.term != nil {
		p.term.Close()
	}
}

func SpawnPane(id uint32, rows, cols int, shell string, parent *actor.Ref) *actor.Ref {
	p := NewPane(id, rows, cols, shell)
	return actor.SpawnWithParent(actor.WithLifecycle(p), 256, parent)
}

func (p *Pane) readLoop() {
	buf := make([]byte, 4096)
	readDone := make(chan error, 1)
	waitDone := make(chan error, 1)

	go func() {
		for {
			n, err := p.pty.TTY.Read(buf)
			if n > 0 && p.self != nil {
				chunk := append([]byte(nil), buf[:n]...)
				p.self.Send(panePTYData{Data: chunk})
			}
			if err != nil {
				readDone <- err
				return
			}
		}
	}()

	go func() {
		waitDone <- p.pty.Wait()
	}()

	var err error
	select {
	case err = <-readDone:
	case err = <-waitDone:
	}

	if p.self != nil {
		p.self.Send(paneProcessExited{Err: err})
	}
}

func (p *Pane) Receive(msg any) {
	switch m := msg.(type) {
	case panePTYData:
		p.term.VTWrite(m.Data)
		p.markDirty()
	case paneFlushUpdate:
		p.updatePending = false
		if parent := actor.Parent(); parent != nil {
			parent.Send(PaneContentUpdated{ID: p.id})
		}
	case paneProcessExited:
		if p.stopped {
			return
		}
		Infof("pane %d: process exited: %v", p.id, m.Err)
		p.stopped = true
		if parent := actor.Parent(); parent != nil {
			parent.Send(PaneExited{ID: p.id})
		}
		if me := actor.Self(); me != nil {
			me.Stop()
		}
	case WriteToPane:
		p.writeToPTY(m.Data)
	case KeyInput:
		p.handleKeyInput(m)
	case ResizeTerm:
		Infof("pane %d: resizing from %dx%d to %dx%d", p.id, p.rows, p.cols, m.Rows, m.Cols)
		p.Resize(m.Rows, m.Cols)
		if p.pty != nil {
			_ = p.pty.Resize(m.Rows, m.Cols)
		}
		p.markDirty()
	case KillPane:
		if p.stopped {
			return
		}
		p.stopped = true
		if parent := actor.Parent(); parent != nil {
			parent.Send(PaneExited{ID: p.id})
		}
		if me := actor.Self(); me != nil {
			me.Stop()
		}
	case actor.AskEnvelope:
		p.handleAsk(m)
	}
}

func (p *Pane) handleAsk(envelope actor.AskEnvelope) {
	switch envelope.Msg.(type) {
	case GetPaneMode:
		mode := &PaneMode{
			InAltScreen:  p.IsAltScreen(),
			CursorHidden: !p.IsCursorVisible(),
		}
		envelope.Reply <- mode
	case GetPaneContent:
		if !p.dirty && p.cachedContent != nil {
			envelope.Reply <- p.cachedContent
			return
		}

		content := p.buildContent()
		p.cachedContent = content
		p.dirty = false
		envelope.Reply <- content
	default:
		envelope.Reply <- nil
	}
}

func (p *Pane) buildContent() *PaneContent {
	content := &PaneContent{
		Lines:          make([]string, p.rows),
		Cells:          make([][]PaneCell, p.rows),
		CursorHidden:   true,
		Title:          p.windowTitle,
		BellCount:      p.bellCount,
		ScrollbackRows: 0,
	}
	if p.term == nil || p.renderState == nil {
		p.fillBlankContent(content)
		return content
	}

	if rows, err := p.term.ScrollbackRows(); err == nil {
		content.ScrollbackRows = rows
	}
	if altScreen, _ := p.term.ModeGet(libghostty.ModeAltScreen); altScreen {
		content.InAltScreen = true
	}

	if err := p.renderState.Update(p.term); err != nil {
		p.fillBlankContent(content)
		return content
	}

	if cursorVisible, err := p.renderState.CursorVisible(); err == nil {
		content.CursorHidden = !cursorVisible
	}
	if hasValue, _ := p.renderState.CursorViewportHasValue(); hasValue {
		if x, err := p.renderState.CursorViewportX(); err == nil {
			content.CursorCol = int(x)
		}
		if y, err := p.renderState.CursorViewportY(); err == nil {
			content.CursorRow = int(y)
		}
		if !content.CursorHidden {
			content.CursorHidden = false
		}
	}

	if err := p.renderState.RowIterator(p.rowIterator); err != nil {
		p.fillBlankContent(content)
		return content
	}

	rowIdx := 0
	for rowIdx < p.rows && p.rowIterator.Next() {
		cells := blankPaneRow(p.cols)
		if err := p.rowIterator.Cells(p.rowCells); err == nil {
			col := 0
			for col < p.cols && p.rowCells.Next() {
				cells[col] = p.currentRowCell()
				col++
			}
		}
		content.Cells[rowIdx] = cells
		content.Lines[rowIdx] = cellsToLine(cells)
		rowIdx++
	}

	for ; rowIdx < p.rows; rowIdx++ {
		cells := blankPaneRow(p.cols)
		content.Cells[rowIdx] = cells
		content.Lines[rowIdx] = cellsToLine(cells)
	}

	return content
}

func (p *Pane) currentRowCell() PaneCell {
	cell := blankPaneCell()

	raw, err := p.rowCells.Raw()
	if err != nil {
		return cell
	}

	if wide, err := raw.Wide(); err == nil {
		switch wide {
		case libghostty.CellWideWide:
			cell.Width = 2
		case libghostty.CellWideSpacerTail, libghostty.CellWideSpacerHead:
			cell.Width = 0
			cell.Text = ""
		default:
			cell.Width = 1
		}
	}

	if graphemes, err := p.rowCells.Graphemes(); err == nil && len(graphemes) > 0 {
		cell.Text = codepointsToString(graphemes)
	} else if cell.Width > 0 {
		cell.Text = " "
	}

	if style, err := p.rowCells.Style(); err == nil {
		cell.Bold = style.Bold()
		cell.Italic = style.Italic()
		cell.Underline = style.Underline() != libghostty.UnderlineNone
		cell.Blink = style.Blink()
		cell.Reverse = style.Inverse()
	}

	if fg, err := p.rowCells.FgColor(); err == nil && fg != nil {
		cell.FgColor = *fg
	}
	if bg, err := p.rowCells.BgColor(); err == nil && bg != nil {
		cell.BgColor = *bg
	}
	if hasHyperlink, err := raw.HasHyperlink(); err == nil {
		cell.HasHyperlink = hasHyperlink
	}

	return cell
}

func (p *Pane) fillBlankContent(content *PaneContent) {
	for row := 0; row < p.rows; row++ {
		cells := blankPaneRow(p.cols)
		content.Cells[row] = cells
		content.Lines[row] = cellsToLine(cells)
	}
}

func (p *Pane) handleKeyInput(input KeyInput) {
	if input.Text != "" && input.Mods&(KeyModCtrl|KeyModAlt|KeyModMeta|KeyModSuper) == 0 {
		p.writeToPTY([]byte(input.Text))
		return
	}

	encoded, err := p.encodeKeyInput(input)
	if err != nil {
		Warnf("pane %d: failed to encode key input: %v", p.id, err)
		return
	}
	if len(encoded) == 0 && input.Text != "" {
		encoded = []byte(input.Text)
	}
	p.writeToPTY(encoded)
}

func (p *Pane) encodeKeyInput(input KeyInput) ([]byte, error) {
	if p.keyEncoder == nil {
		return nil, nil
	}

	event, err := libghostty.NewKeyEvent()
	if err != nil {
		return nil, err
	}
	defer event.Close()

	if input.IsRepeat {
		event.SetAction(libghostty.KeyActionRepeat)
	} else {
		event.SetAction(libghostty.KeyActionPress)
	}
	event.SetMods(ghosttyMods(input.Mods))

	if key := ghosttyKeyFromInput(input); key != libghostty.KeyUnidentified {
		event.SetKey(key)
	}
	if input.Text != "" {
		event.SetUTF8(input.Text)
	}
	if cp := keyInputCodepoint(input); cp != 0 {
		event.SetUnshiftedCodepoint(cp)
	}

	p.keyEncoder.SetOptFromTerminal(p.term)
	return p.keyEncoder.Encode(event)
}

func (p *Pane) writeToPTY(data []byte) {
	if len(data) == 0 || p.pty == nil || p.pty.TTY == nil {
		return
	}
	_, _ = p.pty.TTY.Write(data)
}

func (p *Pane) markDirty() {
	p.dirty = true
	p.cachedContent = nil
	p.scheduleContentUpdate(16 * time.Millisecond)
}

func (p *Pane) scheduleContentUpdate(delay time.Duration) {
	if p.self == nil || p.updatePending {
		return
	}
	p.updatePending = true
	if p.updateTimer != nil {
		p.updateTimer.Stop()
	}
	p.updateTimer = actor.SendAfter(p.self, paneFlushUpdate{}, delay)
}

func (p *Pane) Resize(rows, cols int) {
	if p.term != nil {
		_ = p.term.Resize(uint16(cols), uint16(rows), 0, 0)
	}
	p.rows = rows
	p.cols = cols
}

func (p *Pane) IsAltScreen() bool {
	if p.term == nil {
		return false
	}
	alt, _ := p.term.ModeGet(libghostty.ModeAltScreen)
	return alt
}

func (p *Pane) IsCursorVisible() bool {
	if p.term == nil {
		return false
	}
	visible, _ := p.term.ModeGet(libghostty.ModeCursorVisible)
	return visible
}

func blankPaneCell() PaneCell {
	return PaneCell{Text: " ", Width: 1}
}

func blankPaneRow(cols int) []PaneCell {
	row := make([]PaneCell, cols)
	for i := range row {
		row[i] = blankPaneCell()
	}
	return row
}

func cellsToLine(cells []PaneCell) string {
	var b strings.Builder
	for _, cell := range cells {
		if cell.Width == 0 {
			continue
		}
		if cell.Text == "" {
			b.WriteByte(' ')
			continue
		}
		b.WriteString(cell.Text)
	}
	return b.String()
}

func codepointsToString(codepoints []uint32) string {
	var b strings.Builder
	for _, cp := range codepoints {
		b.WriteRune(rune(cp))
	}
	return b.String()
}

func ghosttyMods(mods KeyMods) libghostty.Mods {
	var result libghostty.Mods
	if mods&KeyModShift != 0 {
		result |= libghostty.ModShift
	}
	if mods&KeyModAlt != 0 {
		result |= libghostty.ModAlt
	}
	if mods&KeyModCtrl != 0 {
		result |= libghostty.ModCtrl
	}
	if mods&KeyModMeta != 0 || mods&KeyModSuper != 0 {
		result |= libghostty.ModSuper
	}
	return result
}

func ghosttyKeyFromInput(input KeyInput) libghostty.Key {
	if key := ghosttyKeyFromCode(input.BaseCode); key != libghostty.KeyUnidentified {
		return key
	}
	if key := ghosttyKeyFromCode(input.Code); key != libghostty.KeyUnidentified {
		return key
	}
	if input.Text != "" {
		r, _ := utf8.DecodeRuneInString(input.Text)
		if key := ghosttyKeyFromCode(r); key != libghostty.KeyUnidentified {
			return key
		}
	}
	return libghostty.KeyUnidentified
}

func ghosttyKeyFromCode(code rune) libghostty.Key {
	switch code {
	case KeyCodeUp:
		return libghostty.KeyArrowUp
	case KeyCodeDown:
		return libghostty.KeyArrowDown
	case KeyCodeRight:
		return libghostty.KeyArrowRight
	case KeyCodeLeft:
		return libghostty.KeyArrowLeft
	case KeyCodeHome:
		return libghostty.KeyHome
	case KeyCodeEnd:
		return libghostty.KeyEnd
	case KeyCodePageUp:
		return libghostty.KeyPageUp
	case KeyCodePageDown:
		return libghostty.KeyPageDown
	case KeyCodeInsert:
		return libghostty.KeyInsert
	case KeyCodeDelete:
		return libghostty.KeyDelete
	case KeyCodeEnter:
		return libghostty.KeyEnter
	case KeyCodeBackspace:
		return libghostty.KeyBackspace
	case KeyCodeTab:
		return libghostty.KeyTab
	case KeyCodeEscape:
		return libghostty.KeyEscape
	case KeyCodeF1:
		return libghostty.KeyF1
	case KeyCodeF2:
		return libghostty.KeyF2
	case KeyCodeF3:
		return libghostty.KeyF3
	case KeyCodeF4:
		return libghostty.KeyF4
	case KeyCodeF5:
		return libghostty.KeyF5
	case KeyCodeF6:
		return libghostty.KeyF6
	case KeyCodeF7:
		return libghostty.KeyF7
	case KeyCodeF8:
		return libghostty.KeyF8
	case KeyCodeF9:
		return libghostty.KeyF9
	case KeyCodeF10:
		return libghostty.KeyF10
	case KeyCodeF11:
		return libghostty.KeyF11
	case KeyCodeF12:
		return libghostty.KeyF12
	case 'a', 'A':
		return libghostty.KeyA
	case 'b', 'B':
		return libghostty.KeyB
	case 'c', 'C':
		return libghostty.KeyC
	case 'd', 'D':
		return libghostty.KeyD
	case 'e', 'E':
		return libghostty.KeyE
	case 'f', 'F':
		return libghostty.KeyF
	case 'g', 'G':
		return libghostty.KeyG
	case 'h', 'H':
		return libghostty.KeyH
	case 'i', 'I':
		return libghostty.KeyI
	case 'j', 'J':
		return libghostty.KeyJ
	case 'k', 'K':
		return libghostty.KeyK
	case 'l', 'L':
		return libghostty.KeyL
	case 'm', 'M':
		return libghostty.KeyM
	case 'n', 'N':
		return libghostty.KeyN
	case 'o', 'O':
		return libghostty.KeyO
	case 'p', 'P':
		return libghostty.KeyP
	case 'q', 'Q':
		return libghostty.KeyQ
	case 'r', 'R':
		return libghostty.KeyR
	case 's', 'S':
		return libghostty.KeyS
	case 't', 'T':
		return libghostty.KeyT
	case 'u', 'U':
		return libghostty.KeyU
	case 'v', 'V':
		return libghostty.KeyV
	case 'w', 'W':
		return libghostty.KeyW
	case 'x', 'X':
		return libghostty.KeyX
	case 'y', 'Y':
		return libghostty.KeyY
	case 'z', 'Z':
		return libghostty.KeyZ
	case '0':
		return libghostty.KeyDigit0
	case '1':
		return libghostty.KeyDigit1
	case '2':
		return libghostty.KeyDigit2
	case '3':
		return libghostty.KeyDigit3
	case '4':
		return libghostty.KeyDigit4
	case '5':
		return libghostty.KeyDigit5
	case '6':
		return libghostty.KeyDigit6
	case '7':
		return libghostty.KeyDigit7
	case '8':
		return libghostty.KeyDigit8
	case '9':
		return libghostty.KeyDigit9
	case '`', '~':
		return libghostty.KeyBackquote
	case '\\', '|':
		return libghostty.KeyBackslash
	case '[', '{':
		return libghostty.KeyBracketLeft
	case ']', '}':
		return libghostty.KeyBracketRight
	case ',':
		return libghostty.KeyComma
	case '=', '+':
		return libghostty.KeyEqual
	case '-', '_':
		return libghostty.KeyMinus
	case '.', '>':
		return libghostty.KeyPeriod
	case '\'', '"':
		return libghostty.KeyQuote
	case ';', ':':
		return libghostty.KeySemicolon
	case '/', '?':
		return libghostty.KeySlash
	case ' ':
		return libghostty.KeySpace
	default:
		return libghostty.KeyUnidentified
	}
}

func keyInputCodepoint(input KeyInput) rune {
	if input.Code >= 0x20 && input.Code < utf8.RuneSelf {
		return input.Code
	}
	if input.Text != "" {
		r, _ := utf8.DecodeRuneInString(input.Text)
		return r
	}
	return 0
}
