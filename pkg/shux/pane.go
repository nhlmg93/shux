package shux

import (
	"os"
	"os/exec"
	"time"

	"github.com/mitchellh/go-libghostty"
	"github.com/nhlmg93/gotor/actor"
)

var _ Resizable = (*Pane)(nil)

type PaneCell struct {
	Text         string
	Width        int
	HasFgColor   bool
	FgColor      libghostty.ColorRGB
	HasBgColor   bool
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
