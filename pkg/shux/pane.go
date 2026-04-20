package shux

import (
	"os/exec"

	"github.com/mitchellh/go-libghostty"
	"github.com/nhlmg93/gotor/actor"
)

var _ Resizable = (*Pane)(nil)

type PaneCell struct {
	Char         rune
	FgColor      libghostty.ColorRGB
	BgColor      libghostty.ColorRGB
	Bold         bool
	Italic       bool
	Underline    bool
	Blink        bool
	Reverse      bool
	HasHyperlink bool
	HyperlinkURL string
}

type PaneContent struct {
	Lines        []string
	Cells        [][]PaneCell
	CursorRow    int
	CursorCol    int
	InAltScreen  bool
	CursorHidden bool
}

type Pane struct {
	id          uint32
	term        *libghostty.Terminal
	renderState *libghostty.RenderState
	pty         *PTY
	rows        int
	cols        int
	shell       string
	windowTitle string
}

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

	ghosttyTerm, err := libghostty.NewTerminal(
		libghostty.WithSize(uint16(p.cols), uint16(p.rows)),
		libghostty.WithMaxScrollback(10000),
		libghostty.WithTitleChanged(func(t *libghostty.Terminal) {}),
		libghostty.WithBell(func(t *libghostty.Terminal) {}),
		libghostty.WithWritePty(func(t *libghostty.Terminal, data []byte) {
			// Terminal sends response data (DSR, etc) - write back to PTY
			if p.pty != nil && p.pty.TTY != nil {
				p.pty.TTY.Write(data)
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

	Infof("pane %d: starting shell %s", p.id, p.shell)
	cmd := exec.Command(p.shell)
	pty, err := Start(cmd)
	if err != nil {
		ghosttyTerm.Close()
		renderState.Close()
		return err
	}

	p.term = ghosttyTerm
	p.renderState = renderState
	p.pty = pty

	go p.readLoop()
	return nil
}

func (p *Pane) Terminate(reason error) {
	Infof("pane %d: terminating (%v)", p.id, reason)
	if p.pty != nil {
		p.pty.Close()
	}
	if p.term != nil {
		p.term.Close()
	}
	if p.renderState != nil {
		p.renderState.Close()
	}
}

func SpawnPane(id uint32, rows, cols int, shell string, parent *actor.Ref) *actor.Ref {
	p := NewPane(id, rows, cols, shell)
	return actor.SpawnWithParent(actor.WithLifecycle(p), 10, parent)
}

func (p *Pane) readLoop() {
	buf := make([]byte, 4096)
	readDone := make(chan error, 1)
	waitDone := make(chan error, 1)

	go func() {
		for {
			n, err := p.pty.TTY.Read(buf)
			if err != nil {
				readDone <- err
				return
			}
			if n > 0 {
				p.term.VTWrite(buf[:n])
				if parent := actor.Parent(); parent != nil {
					parent.Send(PaneContentUpdated{ID: p.id})
				}
			}
		}
	}()

	go func() {
		waitDone <- p.pty.Wait()
	}()

	select {
	case <-readDone:
		p.notifyExited()
	case err := <-waitDone:
		Infof("pane %d: process exited: %v", p.id, err)
		p.notifyExited()
	}
}

func (p *Pane) notifyExited() {
	if parent := actor.Parent(); parent != nil {
		parent.Send(PaneExited{ID: p.id})
	}
}

func (p *Pane) Receive(msg any) {
	switch m := msg.(type) {
	case WriteToPane:
		p.pty.TTY.Write([]byte(m.Data))
	case ResizeTerm:
		Infof("pane %d: resizing from %dx%d to %dx%d", p.id, p.rows, p.cols, m.Rows, m.Cols)
		p.Resize(m.Rows, m.Cols)
		p.pty.Resize(m.Rows, m.Cols)
	case KillPane:
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
		Debugf("pane %d: GetPaneContent p.rows=%d p.cols=%d", p.id, p.rows, p.cols)
		content := &PaneContent{
			Lines: make([]string, p.rows),
			Cells: make([][]PaneCell, p.rows),
		}

		p.renderState.Update(p.term)

		if hasValue, _ := p.renderState.CursorViewportHasValue(); hasValue {
			if x, err := p.renderState.CursorViewportX(); err == nil {
				content.CursorCol = int(x)
			}
			if y, err := p.renderState.CursorViewportY(); err == nil {
				content.CursorRow = int(y)
			}
		}

		for row := 0; row < p.rows; row++ {
			line := make([]rune, p.cols)
			cells := make([]PaneCell, p.cols)
			for col := 0; col < p.cols; col++ {
				cell := p.getCell(row, col)
				cells[col] = cell
				line[col] = cell.Char
			}
			content.Lines[row] = string(line)
			content.Cells[row] = cells
		}

		if altScreen, _ := p.term.ModeGet(libghostty.ModeAltScreen); altScreen {
			content.InAltScreen = true
		}
		if cursorVisible, _ := p.term.ModeGet(libghostty.ModeCursorVisible); !cursorVisible {
			content.CursorHidden = true
		}

		Debugf("pane %d: returning content with %d lines", p.id, len(content.Lines))
		envelope.Reply <- content
	default:
		envelope.Reply <- nil
	}
}

func (p *Pane) getCell(row, col int) PaneCell {
	ref, err := p.term.GridRef(libghostty.Point{
		Tag: libghostty.PointTagActive,
		X:   uint16(col),
		Y:   uint32(row),
	})
	if err != nil {
		return PaneCell{Char: ' '}
	}

	cell, err := ref.Cell()
	if err != nil {
		return PaneCell{Char: ' '}
	}

	result := PaneCell{Char: ' '}
	hasText, _ := cell.HasText()
	if hasText {
		cp, _ := cell.Codepoint()
		result.Char = rune(cp)
	}

	style, err := ref.Style()
	if err == nil {
		result.Bold = style.Bold()
		result.Italic = style.Italic()
		result.Underline = style.Underline() != libghostty.UnderlineNone
		result.Blink = style.Blink()
		result.Reverse = style.Inverse()

		fgColor := style.FgColor()
		if fgColor.Tag == libghostty.StyleColorRGB {
			result.FgColor = fgColor.RGB
		}

		bgColor := style.BgColor()
		if bgColor.Tag == libghostty.StyleColorRGB {
			result.BgColor = bgColor.RGB
		}
	}

	result.HasHyperlink, _ = cell.HasHyperlink()
	return result
}

func (p *Pane) Resize(rows, cols int) {
	p.term.Resize(uint16(cols), uint16(rows), 0, 0)
	p.rows = rows
	p.cols = cols
}

func (p *Pane) IsAltScreen() bool {
	alt, _ := p.term.ModeGet(libghostty.ModeAltScreen)
	return alt
}

func (p *Pane) IsCursorVisible() bool {
	visible, _ := p.term.ModeGet(libghostty.ModeCursorVisible)
	return visible
}
