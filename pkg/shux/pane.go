package shux

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/mitchellh/go-libghostty"
)

var _ Resizable = (*Pane)(nil)

type PaneRef struct {
	*loopRef
}

func (r *PaneRef) Send(msg any) bool {
	if r == nil {
		return false
	}
	return r.send(msg)
}

func (r *PaneRef) Ask(msg any) chan any {
	if r == nil {
		return nil
	}
	return r.ask(msg)
}

func (r *PaneRef) Stop() {
	if r != nil {
		r.stopLoop()
	}
}

func (r *PaneRef) Shutdown() {
	if r != nil {
		r.shutdown()
	}
}

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
	ref           *PaneRef
	parent        *WindowRef
	id            uint32
	term          *libghostty.Terminal
	renderState   *libghostty.RenderState
	rowIterator   *libghostty.RenderStateRowIterator
	rowCells      *libghostty.RenderStateRowCells
	keyEncoder    *libghostty.KeyEncoder
	pty           *PTY
	rows          int
	cols          int
	shell         string
	cwd           string
	windowTitle   string
	bellCount     uint64
	dirty         bool
	cachedContent *PaneContent
	updateTimer   *time.Timer
	updatePending bool
	stopped       bool
}

type (
	panePTYData       struct{ Data []byte }
	paneFlushUpdate   struct{}
	paneProcessExited struct{ Err error }
)

func NewPane(id uint32, rows, cols int, shell, cwd string) *Pane {
	originalRows, originalCols := rows, cols
	rows, cols, changed := sanitizeTermSize(rows, cols)
	if changed {
		Warnf("pane: id=%d sanitize-size from=%dx%d to=%dx%d", id, originalRows, originalCols, rows, cols)
	}
	return &Pane{
		id:    id,
		rows:  rows,
		cols:  cols,
		shell: shell,
		cwd:   cwd,
	}
}

func StartPane(id uint32, rows, cols int, shell, cwd string, parent *WindowRef) *PaneRef {
	p := NewPane(id, rows, cols, shell, cwd)
	p.parent = parent
	ref := &PaneRef{loopRef: newLoopRef(256)}
	p.ref = ref
	go p.run()
	return ref
}

func (p *Pane) run() {
	var reason error
	defer func() {
		if r := recover(); r != nil {
			reason = fmt.Errorf("panic: %v", r)
		}
		p.terminate(reason)
		close(p.ref.done)
	}()

	if err := p.init(); err != nil {
		Errorf("pane: id=%d init failed err=%v", p.id, err)
		if p.parent != nil {
			p.parent.Send(PaneExited{ID: p.id})
		}
		reason = err
		return
	}

	for {
		select {
		case <-p.ref.stop:
			return
		case msg := <-p.ref.inbox:
			p.receive(msg)
		}
	}
}

func (p *Pane) init() error {
	Infof("pane: id=%d init shell=%s cwd=%s rows=%d cols=%d", p.id, p.shell, p.cwd, p.rows, p.cols)

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

	Infof("pane: id=%d spawn shell=%s cwd=%s", p.id, p.shell, p.cwd)
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
	if p.cwd != "" {
		cmd.Dir = ResolveCWD(p.cwd)
	}

	pty, err := StartWithSize(cmd, p.rows, p.cols)
	if err != nil {
		keyEncoder.Close()
		rowCells.Close()
		rowIterator.Close()
		renderState.Close()
		ghosttyTerm.Close()
		return err
	}

	Infof("pane: id=%d started pid=%d shell=%s cwd=%s rows=%d cols=%d", p.id, pty.PID(), p.shell, cmd.Dir, p.rows, p.cols)
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

func (p *Pane) terminate(reason error) {
	if reason != nil {
		Errorf("pane: crash id=%d pid=%d reason=%v", p.id, p.pid(), reason)
	} else {
		Infof("pane: terminate id=%d pid=%d", p.id, p.pid())
	}
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

func (p *Pane) readLoop() {
	buf := make([]byte, 4096)
	readDone := make(chan error, 1)
	waitDone := make(chan error, 1)

	go func() {
		for {
			n, err := p.pty.TTY.Read(buf)
			if n > 0 && p.ref != nil {
				chunk := append([]byte(nil), buf[:n]...)
				p.ref.Send(panePTYData{Data: chunk})
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

	if p.ref != nil {
		p.ref.Send(paneProcessExited{Err: err})
	}
}

func (p *Pane) receive(msg any) {
	switch m := msg.(type) {
	case panePTYData:
		p.term.VTWrite(m.Data)
		p.markDirty()
	case paneFlushUpdate:
		p.updatePending = false
		if p.parent != nil {
			p.parent.Send(PaneContentUpdated{ID: p.id})
		}
	case paneProcessExited:
		if p.stopped {
			return
		}
		Infof("pane: id=%d process-exited pid=%d err=%v", p.id, p.pid(), m.Err)
		p.stopped = true
		if p.parent != nil {
			p.parent.Send(PaneExited{ID: p.id})
		}
		p.ref.Stop()
	case WriteToPane:
		p.writeToPTY(m.Data)
	case KeyInput:
		p.handleKeyInput(m)
	case ResizeTerm:
		Infof("pane: id=%d resize from=%dx%d to=%dx%d", p.id, p.rows, p.cols, m.Rows, m.Cols)
		p.Resize(m.Rows, m.Cols)
		if p.pty != nil {
			_ = p.pty.Resize(m.Rows, m.Cols)
		}
		p.markDirty()
	case KillPane:
		if p.stopped {
			return
		}
		Infof("pane: id=%d kill requested pid=%d", p.id, p.pid())
		p.stopped = true
		if p.parent != nil {
			p.parent.Send(PaneExited{ID: p.id})
		}
		p.ref.Stop()
	case askEnvelope:
		p.handleAsk(m)
	}
}

func (p *Pane) handleAsk(envelope askEnvelope) {
	switch envelope.msg.(type) {
	case GetPaneMode:
		envelope.reply <- &PaneMode{
			InAltScreen:  p.IsAltScreen(),
			CursorHidden: !p.IsCursorVisible(),
		}
	case GetPaneContent:
		if !p.dirty && p.cachedContent != nil {
			envelope.reply <- p.cachedContent
			return
		}
		content := p.buildContent()
		p.cachedContent = content
		p.dirty = false
		envelope.reply <- content
	case GetPaneShell:
		envelope.reply <- p.shell
	case GetPaneSnapshotData:
		envelope.reply <- PaneSnapshotData{
			ID:          p.id,
			Shell:       p.shell,
			Rows:        p.rows,
			Cols:        p.cols,
			CWD:         p.getCWD(),
			WindowTitle: p.windowTitle,
		}
	default:
		envelope.reply <- nil
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
	if p.ref == nil || p.updatePending {
		return
	}
	p.updatePending = true
	if p.updateTimer != nil {
		p.updateTimer.Stop()
	}
	p.updateTimer = time.AfterFunc(delay, func() {
		if p.ref != nil {
			p.ref.Send(paneFlushUpdate{})
		}
	})
}

func (p *Pane) Resize(rows, cols int) {
	originalRows, originalCols := rows, cols
	rows, cols, changed := sanitizeTermSize(rows, cols)
	if changed {
		Warnf("pane: id=%d sanitize-resize from=%dx%d to=%dx%d", p.id, originalRows, originalCols, rows, cols)
	}
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

func (p *Pane) pid() int {
	if p.pty == nil {
		return 0
	}
	return p.pty.PID()
}

func (p *Pane) getCWD() string {
	if p.pty == nil {
		return ""
	}

	pid := p.pty.PID()
	if pid == 0 {
		return ""
	}

	cwd, err := GetProcessCWD(pid)
	if err != nil {
		return ""
	}

	return ResolveCWD(cwd)
}
