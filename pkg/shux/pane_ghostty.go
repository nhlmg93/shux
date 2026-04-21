package shux

import "github.com/mitchellh/go-libghostty"

// createTerminal creates the Ghostty terminal and supporting objects.
func (pr *PaneRuntime) createTerminal() error {
	var err error

	pr.term, err = libghostty.NewTerminal(
		libghostty.WithSize(uint16(pr.cols), uint16(pr.rows)),
		libghostty.WithMaxScrollback(10000),
		libghostty.WithTitleChanged(func(t *libghostty.Terminal) {
			if title, err := t.Title(); err == nil {
				pr.mu.Lock()
				pr.windowTitle = title
				pr.mu.Unlock()
				if pr.onTitleChanged != nil {
					pr.onTitleChanged(title)
				}
			}
		}),
		libghostty.WithBell(func(t *libghostty.Terminal) {
			pr.mu.Lock()
			pr.bellCount++
			pr.mu.Unlock()
			if pr.onBell != nil {
				pr.onBell()
			}
		}),
		libghostty.WithWritePty(func(t *libghostty.Terminal, data []byte) {
			pr.mu.RLock()
			pty := pr.pty
			pr.mu.RUnlock()
			if pty != nil {
				_, _ = pty.Write(data)
			}
		}),
	)
	if err != nil {
		return err
	}

	pr.renderState, err = libghostty.NewRenderState()
	if err != nil {
		pr.term.Close()
		pr.term = nil
		return err
	}

	pr.rowIterator, err = libghostty.NewRenderStateRowIterator()
	if err != nil {
		pr.renderState.Close()
		pr.renderState = nil
		pr.term.Close()
		pr.term = nil
		return err
	}

	pr.rowCells, err = libghostty.NewRenderStateRowCells()
	if err != nil {
		pr.rowIterator.Close()
		pr.rowIterator = nil
		pr.renderState.Close()
		pr.renderState = nil
		pr.term.Close()
		pr.term = nil
		return err
	}

	pr.keyEncoder, err = libghostty.NewKeyEncoder()
	if err != nil {
		pr.rowCells.Close()
		pr.rowCells = nil
		pr.rowIterator.Close()
		pr.rowIterator = nil
		pr.renderState.Close()
		pr.renderState = nil
		pr.term.Close()
		pr.term = nil
		return err
	}

	pr.mouseEncoder, err = libghostty.NewMouseEncoder()
	if err != nil {
		pr.keyEncoder.Close()
		pr.keyEncoder = nil
		pr.rowCells.Close()
		pr.rowCells = nil
		pr.rowIterator.Close()
		pr.rowIterator = nil
		pr.renderState.Close()
		pr.renderState = nil
		pr.term.Close()
		pr.term = nil
		return err
	}
	pr.mouseEncoder.SetOptTrackLastCell(true)

	return nil
}

// cleanupTerminal closes terminal-related resources.
func (pr *PaneRuntime) cleanupTerminal() {
	if pr.mouseEncoder != nil {
		pr.mouseEncoder.Close()
		pr.mouseEncoder = nil
	}
	if pr.keyEncoder != nil {
		pr.keyEncoder.Close()
		pr.keyEncoder = nil
	}
	if pr.rowCells != nil {
		pr.rowCells.Close()
		pr.rowCells = nil
	}
	if pr.rowIterator != nil {
		pr.rowIterator.Close()
		pr.rowIterator = nil
	}
	if pr.renderState != nil {
		pr.renderState.Close()
		pr.renderState = nil
	}
	if pr.term != nil {
		pr.term.Close()
		pr.term = nil
	}
}

// RenderState returns the render state for building content.
func (pr *PaneRuntime) RenderState() *libghostty.RenderState {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.renderState
}

// Term returns the underlying terminal (for advanced operations).
func (pr *PaneRuntime) Term() *libghostty.Terminal {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.term
}

// KeyEncoder returns the key encoder.
func (pr *PaneRuntime) KeyEncoder() *libghostty.KeyEncoder {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.keyEncoder
}

// MouseEncoder returns the mouse encoder.
func (pr *PaneRuntime) MouseEncoder() *libghostty.MouseEncoder {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.mouseEncoder
}
