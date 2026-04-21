package shux

import (
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mitchellh/go-libghostty"
)

// PaneController owns pane coordination and can be restarted around a stable runtime.
// It does NOT own the PTY/process - that belongs to PaneRuntime.
// If the controller panics, it can be restarted without killing the shell.
type PaneController struct {
	ref     *PaneRef
	runtime *PaneRuntime // The stable runtime (survives controller restarts)
	parent  *WindowRef
	id      uint32

	// Controller-local state
	mouseButtons map[MouseButton]bool
	contentCache paneContentCache
	stopped      bool

	// Callback management - prevents callbacks after stop
	callbackMu    sync.RWMutex
	callbacksDone bool

	logger ShuxLogger
}

// paneContentCache caches the rendered content with debounced updates.
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

// PaneRef is a reference to a pane controller loop. Methods are promoted from loopRef.
type PaneRef struct {
	*loopRef
}

// Internal message types for the pane controller.
type (
	paneFlushUpdate   struct{}
	paneProcessExited struct{ Err error }
)

// NewPaneController creates a new controller around an existing runtime.
// This is the primary constructor for pane controllers.
func NewPaneController(id uint32, runtime *PaneRuntime, parent *WindowRef, logger ShuxLogger) *PaneController {
	return &PaneController{
		id:           id,
		runtime:      runtime,
		parent:       parent,
		logger:       logger,
		mouseButtons: make(map[MouseButton]bool),
	}
}

// StartPaneController starts a pane controller loop around an existing runtime.
// This is used when creating new panes or restarting controllers.
func StartPaneController(id uint32, runtime *PaneRuntime, parent *WindowRef, logger ShuxLogger) *PaneRef {
	p := NewPaneController(id, runtime, parent, logger)
	ref := &PaneRef{loopRef: newLoopRef(256)}
	p.ref = ref
	go p.run()
	return ref
}

// run is the main event loop for the pane controller.
func (p *PaneController) run() {
	var reason error
	defer func() {
		if r := recover(); r != nil {
			reason = fmt.Errorf("panic: %v\n%s", r, recoverWithContext("pane_controller", p.id, 0, 0))
		}
		p.terminate(reason)
		close(p.ref.done)
	}()

	// Set up callbacks on the runtime
	p.setupRuntimeCallbacks()

	// Schedule initial update
	p.contentCache.Invalidate()
	p.contentCache.Schedule(p.ref, 0)

	for {
		select {
		case <-p.ref.stop:
			return
		case msg := <-p.ref.inbox:
			p.receive(msg)
		}
	}
}

// setupRuntimeCallbacks configures the runtime to notify the controller of events.
func (p *PaneController) setupRuntimeCallbacks() {
	// The runtime calls these when events occur
	// We wrap them to send messages to our inbox, but check if stopped first
	p.runtime.onTitleChanged = func(title string) {
		p.callbackMu.RLock()
		done := p.callbacksDone
		p.callbackMu.RUnlock()
		if done {
			return
		}
		p.markDirty()
	}
	p.runtime.onBell = func() {
		p.callbackMu.RLock()
		done := p.callbacksDone
		p.callbackMu.RUnlock()
		if done {
			return
		}
		p.markDirty()
	}
	p.runtime.onOutput = func() {
		p.callbackMu.RLock()
		done := p.callbacksDone
		p.callbackMu.RUnlock()
		if done {
			return
		}
		p.markDirty()
	}
	p.runtime.onProcessExit = func(err error) {
		p.callbackMu.RLock()
		done := p.callbacksDone
		p.callbackMu.RUnlock()
		if done {
			return
		}
		p.ref.Send(paneProcessExited{Err: err})
	}
}

// detachRuntimeCallbacks clears the runtime callbacks to prevent further invocations.
func (p *PaneController) detachRuntimeCallbacks() {
	if p.runtime == nil {
		return
	}
	p.runtime.onTitleChanged = nil
	p.runtime.onBell = nil
	p.runtime.onOutput = nil
	p.runtime.onProcessExit = nil
}

// terminate handles cleanup when the controller exits.
// NOTE: This does NOT close the runtime/PTY - that's separate lifecycle.
func (p *PaneController) terminate(reason error) {
	if reason != nil {
		p.logger.Errorf("pane_controller: crash id=%d reason=%v", p.id, reason)
	} else {
		p.logger.Infof("pane_controller: terminate id=%d", p.id)
	}
	p.contentCache.Stop()

	// Mark callbacks as done to prevent further inbox sends
	p.callbackMu.Lock()
	p.callbacksDone = true
	p.callbackMu.Unlock()

	// Detach from runtime callbacks to prevent further invocations
	p.detachRuntimeCallbacks()
}

// receive handles incoming messages.
func (p *PaneController) receive(msg any) {
	switch m := msg.(type) {
	case paneFlushUpdate:
		p.contentCache.ClearPending()
		if p.parent != nil {
			p.parent.Send(PaneContentUpdated{ID: p.id})
		}
	case paneProcessExited:
		if p.stopped {
			p.ref.Stop()
			return
		}
		p.logger.Infof("pane_controller: id=%d process-exited", p.id)
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
		oldRows, oldCols := p.runtime.GetSize()
		p.logger.Infof("pane_controller: id=%d resize from=%dx%d to=%dx%d", p.id, oldRows, oldCols, m.Rows, m.Cols)
		if err := p.runtime.Resize(m.Rows, m.Cols); err != nil {
			p.logger.Warnf("pane_controller: id=%d resize failed: %v", p.id, err)
		}
		p.markDirty()
	case KillPane:
		if p.stopped {
			return
		}
		p.logger.Infof("pane_controller: id=%d kill requested", p.id)
		p.stopped = true
		if p.parent != nil {
			p.parent.Send(PaneExited{ID: p.id})
		}
		go func() {
			if err := p.runtime.Kill(); err != nil {
				p.logger.Warnf("pane_controller: id=%d kill failed: %v", p.id, err)
			}
			p.ref.Stop()
		}()
	case askEnvelope:
		p.handleAsk(m)
	}
}

// handleAsk handles synchronous queries.
func (p *PaneController) handleAsk(envelope askEnvelope) {
	switch envelope.msg.(type) {
	case GetPaneMode:
		envelope.reply <- &PaneMode{
			InAltScreen:  p.runtime.IsAltScreen(),
			CursorHidden: !p.runtime.IsCursorVisible(),
		}
	case GetPaneContent:
		if content, ok := p.contentCache.Current(); ok {
			envelope.reply <- content
			return
		}
		content := p.runtime.BuildContent()
		envelope.reply <- p.contentCache.Store(content)
	case GetPaneShell:
		envelope.reply <- p.runtime.shell
	case GetPaneSnapshotData:
		envelope.reply <- p.runtime.GetSnapshotData()
	default:
		envelope.reply <- nil
	}
}

// writeToPTY writes data to the PTY via the runtime.
func (p *PaneController) writeToPTY(data []byte) {
	if len(data) == 0 {
		return
	}
	_, _ = p.runtime.Write(data)
}

// handleKeyInput processes keyboard input using Ghostty's key encoding API.
func (p *PaneController) handleKeyInput(input KeyInput) {
	encoder := p.runtime.KeyEncoder()
	if encoder == nil {
		return
	}

	// Simple text input without modifiers
	if input.Text != "" && input.Mods&(KeyModCtrl|KeyModAlt|KeyModMeta|KeyModSuper) == 0 {
		p.writeToPTY([]byte(input.Text))
		return
	}

	// Create key event
	event, err := libghostty.NewKeyEvent()
	if err != nil {
		p.logger.Warnf("pane_controller: id=%d failed to create key event: %v", p.id, err)
		return
	}
	defer event.Close()

	// Set action
	if input.IsRepeat {
		event.SetAction(libghostty.KeyActionRepeat)
	} else {
		event.SetAction(libghostty.KeyActionPress)
	}

	// Set modifiers
	event.SetMods(p.ghosttyMods(input.Mods))

	// Set key
	if key := p.ghosttyKeyFromInput(input); key != libghostty.KeyUnidentified {
		event.SetKey(key)
	}

	// Set UTF8 text
	if input.Text != "" {
		event.SetUTF8(input.Text)
	}

	// Set unshifted codepoint
	if cp := p.keyInputCodepoint(input); cp != 0 {
		event.SetUnshiftedCodepoint(cp)
	}

	// Configure encoder and encode
	encoder.SetOptFromTerminal(p.runtime.Term())
	encoded, err := encoder.Encode(event)
	if err != nil {
		p.logger.Warnf("pane_controller: id=%d failed to encode key: %v", p.id, err)
		return
	}

	if len(encoded) == 0 && input.Text != "" {
		encoded = []byte(input.Text)
	}

	p.writeToPTY(encoded)
}

// ghosttyMods converts KeyMods to libghostty.Mods.
func (p *PaneController) ghosttyMods(mods KeyMods) libghostty.Mods {
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

// ghosttyKeyFromInput extracts a Ghostty key from input.
func (p *PaneController) ghosttyKeyFromInput(input KeyInput) libghostty.Key {
	if key := p.ghosttyKeyFromCode(input.BaseCode); key != libghostty.KeyUnidentified {
		return key
	}
	if key := p.ghosttyKeyFromCode(input.Code); key != libghostty.KeyUnidentified {
		return key
	}
	if input.Text != "" {
		r, _ := utf8.DecodeRuneInString(input.Text)
		if key := p.ghosttyKeyFromCode(r); key != libghostty.KeyUnidentified {
			return key
		}
	}
	return libghostty.KeyUnidentified
}

// ghosttyKeyFromCode converts a rune to a Ghostty key.
func (p *PaneController) ghosttyKeyFromCode(code rune) libghostty.Key {
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

// keyInputCodepoint extracts the codepoint from key input.
func (p *PaneController) keyInputCodepoint(input KeyInput) rune {
	if input.Code >= 0x20 && input.Code < utf8.RuneSelf {
		return input.Code
	}
	if input.Text != "" {
		r, _ := utf8.DecodeRuneInString(input.Text)
		return r
	}
	return 0
}

// handleMouseInput processes mouse input using Ghostty's mouse encoding API.
func (p *PaneController) handleMouseInput(input MouseInput) {
	encoder := p.runtime.MouseEncoder()
	if encoder == nil {
		return
	}

	event, err := libghostty.NewMouseEvent()
	if err != nil {
		p.logger.Warnf("pane_controller: id=%d failed to create mouse event: %v", p.id, err)
		return
	}
	defer event.Close()

	// Set action
	switch input.Action {
	case MouseActionPress:
		event.SetAction(libghostty.MouseActionPress)
		if p.tracksMouseButton(input.Button) {
			p.mouseButtons[input.Button] = true
		}
	case MouseActionRelease:
		event.SetAction(libghostty.MouseActionRelease)
		delete(p.mouseButtons, input.Button)
	case MouseActionMotion:
		event.SetAction(libghostty.MouseActionMotion)
		if p.tracksMouseButton(input.Button) {
			p.mouseButtons[input.Button] = true
		}
	default:
		return
	}

	// Set button
	if button, ok := p.ghosttyMouseButton(input.Button); ok {
		event.SetButton(button)
	} else {
		event.ClearButton()
	}

	// Set modifiers
	event.SetMods(p.ghosttyMods(input.Mods))

	// Set position
	event.SetPosition(libghostty.MousePosition{X: float32(input.Col), Y: float32(input.Row)})

	// Configure encoder
	encoder.SetOptFromTerminal(p.runtime.Term())
	rows, cols := p.runtime.GetSize()
	encoder.SetOptSize(libghostty.MouseEncoderSize{
		ScreenWidth:  uint32(max(1, cols)),
		ScreenHeight: uint32(max(1, rows)),
		CellWidth:    1,
		CellHeight:   1,
	})
	encoder.SetOptAnyButtonPressed(len(p.mouseButtons) > 0)

	// Encode and send
	encoded, err := encoder.Encode(event)
	if err != nil {
		p.logger.Warnf("pane_controller: id=%d failed to encode mouse: %v", p.id, err)
		return
	}
	if len(encoded) == 0 {
		return
	}

	p.writeToPTY(encoded)
}

// tracksMouseButton returns true if the button should be tracked.
func (p *PaneController) tracksMouseButton(button MouseButton) bool {
	switch button {
	case MouseButtonLeft, MouseButtonMiddle, MouseButtonRight,
		MouseButtonBackward, MouseButtonForward, MouseButtonButton10, MouseButtonButton11:
		return true
	default:
		return false
	}
}

// ghosttyMouseButton converts MouseButton to libghostty.MouseButton.
func (p *PaneController) ghosttyMouseButton(button MouseButton) (libghostty.MouseButton, bool) {
	switch button {
	case MouseButtonLeft:
		return libghostty.MouseButtonLeft, true
	case MouseButtonMiddle:
		return libghostty.MouseButtonMiddle, true
	case MouseButtonRight:
		return libghostty.MouseButtonRight, true
	case MouseButtonWheelUp:
		return libghostty.MouseButtonFour, true
	case MouseButtonWheelDown:
		return libghostty.MouseButtonFive, true
	case MouseButtonWheelLeft:
		return libghostty.MouseButtonSix, true
	case MouseButtonWheelRight:
		return libghostty.MouseButtonSeven, true
	case MouseButtonBackward:
		return libghostty.MouseButtonEight, true
	case MouseButtonForward:
		return libghostty.MouseButtonNine, true
	case MouseButtonButton10:
		return libghostty.MouseButtonTen, true
	case MouseButtonButton11:
		return libghostty.MouseButtonEleven, true
	default:
		return libghostty.MouseButtonUnknown, false
	}
}

// markDirty marks the content cache dirty and schedules an update.
// Silently does nothing if the controller is stopped.
func (p *PaneController) markDirty() {
	p.callbackMu.RLock()
	stopped := p.callbacksDone || p.stopped
	p.callbackMu.RUnlock()
	if stopped {
		return
	}
	p.contentCache.Invalidate()
	p.contentCache.Schedule(p.ref, 16*time.Millisecond)
}

// Runtime returns the underlying runtime (for access by other components).
func (p *PaneController) Runtime() *PaneRuntime {
	return p.runtime
}

// IsStopped returns true if the controller has been stopped.
func (p *PaneController) IsStopped() bool {
	return p.stopped
}
