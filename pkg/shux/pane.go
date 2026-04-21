package shux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mitchellh/go-libghostty"
)

var _ Resizable = (*Pane)(nil)

// PaneRef is a reference to a pane loop. Methods are promoted from loopRef.
type PaneRef struct {
	*loopRef
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

type paneContentCache struct {
	dirty         bool
	cached        *PaneContent
	updateTimer   *time.Timer
	updatePending bool
}

func (c *paneContentCache) Stop() {
	if c.updateTimer != nil {
		c.updateTimer.Stop()
	}
}

func (c *paneContentCache) Invalidate() {
	c.dirty = true
	c.cached = nil
}

func (c *paneContentCache) ClearPending() {
	c.updatePending = false
}

func (c *paneContentCache) Current() (*PaneContent, bool) {
	if c.dirty || c.cached == nil {
		return nil, false
	}
	return c.cached, true
}

func (c *paneContentCache) Store(content *PaneContent) *PaneContent {
	c.cached = content
	c.dirty = false
	return content
}

func (c *paneContentCache) Schedule(ref *PaneRef, delay time.Duration) {
	if ref == nil || c.updatePending {
		return
	}
	c.updatePending = true
	c.Stop()
	c.updateTimer = time.AfterFunc(delay, func() {
		if ref != nil {
			ref.Send(paneFlushUpdate{})
		}
	})
}

type paneRuntime struct {
	term         *libghostty.Terminal
	renderState  *libghostty.RenderState
	rowIterator  *libghostty.RenderStateRowIterator
	rowCells     *libghostty.RenderStateRowCells
	keyEncoder   *libghostty.KeyEncoder
	mouseEncoder *libghostty.MouseEncoder
	pty          Pty
}

func (r *paneRuntime) Close() {
	if r.pty != nil {
		_ = r.pty.Close()
	}
	if r.mouseEncoder != nil {
		r.mouseEncoder.Close()
	}
	if r.keyEncoder != nil {
		r.keyEncoder.Close()
	}
	if r.rowCells != nil {
		r.rowCells.Close()
	}
	if r.rowIterator != nil {
		r.rowIterator.Close()
	}
	if r.renderState != nil {
		r.renderState.Close()
	}
	if r.term != nil {
		r.term.Close()
	}
}

func (r *paneRuntime) install(p *Pane) {
	p.term = r.term
	p.renderState = r.renderState
	p.rowIterator = r.rowIterator
	p.rowCells = r.rowCells
	p.keyEncoder = r.keyEncoder
	p.mouseEncoder = r.mouseEncoder
	p.pty = r.pty
}

type Pane struct {
	ref          *PaneRef
	logger       ShuxLogger
	parent       *WindowRef
	id           uint32
	term         *libghostty.Terminal
	renderState  *libghostty.RenderState
	rowIterator  *libghostty.RenderStateRowIterator
	rowCells     *libghostty.RenderStateRowCells
	keyEncoder   *libghostty.KeyEncoder
	mouseEncoder *libghostty.MouseEncoder
	pty          Pty
	mouseButtons map[MouseButton]bool
	rows         int
	cols         int
	shell        string
	cwd          string
	windowTitle  string
	bellCount    uint64
	contentCache paneContentCache
	stopped      bool
}

type (
	panePTYData       struct{ Data []byte }
	paneFlushUpdate   struct{}
	paneProcessExited struct{ Err error }
)

func NewPane(id uint32, rows, cols int, shell, cwd string, logger ShuxLogger) *Pane {
	originalRows, originalCols := rows, cols
	rows, cols, changed := sanitizeTermSize(rows, cols)
	if changed {
		logger.Warnf("pane: id=%d sanitize-size from=%dx%d to=%dx%d", id, originalRows, originalCols, rows, cols)
	}
	return &Pane{
		id:           id,
		rows:         rows,
		cols:         cols,
		shell:        shell,
		cwd:          cwd,
		logger:       logger,
		mouseButtons: make(map[MouseButton]bool),
	}
}

func StartPane(id uint32, rows, cols int, shell, cwd string, parent *WindowRef, logger ShuxLogger) *PaneRef {
	p := NewPane(id, rows, cols, shell, cwd, logger)
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
		p.logger.Errorf("pane: id=%d init failed err=%v", p.id, err)
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
	p.logger.Infof("pane: id=%d init shell=%s cwd=%s rows=%d cols=%d", p.id, p.shell, p.cwd, p.rows, p.cols)

	runtime, cmd, err := p.newRuntime()
	if err != nil {
		return err
	}

	p.logger.Infof("pane: id=%d started pid=%d shell=%s cwd=%s rows=%d cols=%d", p.id, runtime.pty.PID(), p.shell, cmd.Dir, p.rows, p.cols)
	runtime.install(p)
	p.contentCache.Invalidate()

	go p.readLoop()
	p.contentCache.Schedule(p.ref, 0)
	return nil
}

func (p *Pane) newRuntime() (_ *paneRuntime, cmd *exec.Cmd, err error) {
	runtime := &paneRuntime{}
	defer func() {
		if err != nil {
			runtime.Close()
		}
	}()

	runtime.term, err = p.newTerminal()
	if err != nil {
		return nil, nil, err
	}
	runtime.renderState, err = libghostty.NewRenderState()
	if err != nil {
		return nil, nil, err
	}
	runtime.rowIterator, err = libghostty.NewRenderStateRowIterator()
	if err != nil {
		return nil, nil, err
	}
	runtime.rowCells, err = libghostty.NewRenderStateRowCells()
	if err != nil {
		return nil, nil, err
	}
	runtime.keyEncoder, err = libghostty.NewKeyEncoder()
	if err != nil {
		return nil, nil, err
	}
	runtime.mouseEncoder, err = libghostty.NewMouseEncoder()
	if err != nil {
		return nil, nil, err
	}
	runtime.mouseEncoder.SetOptTrackLastCell(true)

	p.logger.Infof("pane: id=%d spawn shell=%s cwd=%s", p.id, p.shell, p.cwd)
	cmd = p.newCommand()
	runtime.pty, err = StartWithSize(cmd, p.rows, p.cols)
	if err != nil {
		return nil, nil, err
	}
	return runtime, cmd, nil
}

func (p *Pane) newTerminal() (*libghostty.Terminal, error) {
	return libghostty.NewTerminal(
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
			if p.pty != nil {
				_, _ = p.pty.Write(data)
			}
		}),
	)
}

func (p *Pane) newCommand() *exec.Cmd {
	cmd := exec.Command(p.shell)
	cmd.Env = ensureTermEnv(os.Environ())
	if p.cwd != "" {
		cmd.Dir = ResolveCWD(p.cwd)
	}
	return cmd
}

func ensureTermEnv(env []string) []string {
	for _, entry := range env {
		if strings.HasPrefix(entry, "TERM=") {
			return env
		}
	}
	return append(env, "TERM=xterm-256color")
}

func (p *Pane) terminate(reason error) {
	if reason != nil {
		p.logger.Errorf("pane: crash id=%d pid=%d reason=%v", p.id, p.pid(), reason)
	} else {
		p.logger.Infof("pane: terminate id=%d pid=%d", p.id, p.pid())
	}
	p.contentCache.Stop()
	(&paneRuntime{
		term:         p.term,
		renderState:  p.renderState,
		rowIterator:  p.rowIterator,
		rowCells:     p.rowCells,
		keyEncoder:   p.keyEncoder,
		mouseEncoder: p.mouseEncoder,
		pty:          p.pty,
	}).Close()
}

func (p *Pane) readLoop() {
	buf := make([]byte, 4096)
	readDone := make(chan error, 1)
	waitDone := make(chan error, 1)

	go func() {
		for {
			n, err := p.pty.Read(buf)
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
		p.contentCache.ClearPending()
		if p.parent != nil {
			p.parent.Send(PaneContentUpdated{ID: p.id})
		}
	case paneProcessExited:
		if p.stopped {
			return
		}
		p.logger.Infof("pane: id=%d process-exited pid=%d err=%v", p.id, p.pid(), m.Err)
		p.stopped = true
		if p.parent != nil {
			p.parent.Send(PaneExited{ID: p.id})
		}
		p.ref.Stop()
	case WriteToPane:
		p.writeToPTY(m.Data)
	case KeyInput:
		p.handleKeyInput(m)
	case MouseInput:
		p.handleMouseInput(m)
	case ResizeTerm:
		p.logger.Infof("pane: id=%d resize from=%dx%d to=%dx%d", p.id, p.rows, p.cols, m.Rows, m.Cols)
		p.Resize(m.Rows, m.Cols)
		if p.pty != nil {
			_ = p.pty.Resize(m.Rows, m.Cols)
		}
		p.markDirty()
	case KillPane:
		if p.stopped {
			return
		}
		p.logger.Infof("pane: id=%d kill requested pid=%d", p.id, p.pid())
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
		if content, ok := p.contentCache.Current(); ok {
			envelope.reply <- content
			return
		}
		envelope.reply <- p.contentCache.Store(p.buildContent())
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
	if len(data) == 0 || p.pty == nil {
		return
	}
	_, _ = p.pty.Write(data)
}

func (p *Pane) markDirty() {
	p.contentCache.Invalidate()
	p.contentCache.Schedule(p.ref, 16*time.Millisecond)
}

func (p *Pane) Resize(rows, cols int) {
	originalRows, originalCols := rows, cols
	rows, cols, changed := sanitizeTermSize(rows, cols)
	if changed {
		p.logger.Warnf("pane: id=%d sanitize-resize from=%dx%d to=%dx%d", p.id, originalRows, originalCols, rows, cols)
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
