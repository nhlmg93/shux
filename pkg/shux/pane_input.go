package shux

import (
	"unicode/utf8"

	"github.com/mitchellh/go-libghostty"
)

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

	if input.Text != "" && input.Mods&(KeyModCtrl|KeyModAlt|KeyModMeta|KeyModSuper) == 0 {
		p.writeToPTY([]byte(input.Text))
		return
	}

	event, err := libghostty.NewKeyEvent()
	if err != nil {
		p.logger.Warnf("pane_controller: id=%d failed to create key event: %v", p.id, err)
		return
	}
	defer event.Close()

	if input.IsRepeat {
		event.SetAction(libghostty.KeyActionRepeat)
	} else {
		event.SetAction(libghostty.KeyActionPress)
	}

	event.SetMods(p.ghosttyMods(input.Mods))

	if key := p.ghosttyKeyFromInput(input); key != libghostty.KeyUnidentified {
		event.SetKey(key)
	}

	if input.Text != "" {
		event.SetUTF8(input.Text)
	}

	if cp := p.keyInputCodepoint(input); cp != 0 {
		event.SetUnshiftedCodepoint(cp)
	}

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

	if button, ok := p.ghosttyMouseButton(input.Button); ok {
		event.SetButton(button)
	} else {
		event.ClearButton()
	}

	event.SetMods(p.ghosttyMods(input.Mods))

	event.SetPosition(libghostty.MousePosition{X: float32(input.Col), Y: float32(input.Row)})

	encoder.SetOptFromTerminal(p.runtime.Term())
	rows, cols := p.runtime.GetSize()
	encoder.SetOptSize(libghostty.MouseEncoderSize{
		ScreenWidth:  uint32(max(1, cols)),
		ScreenHeight: uint32(max(1, rows)),
		CellWidth:    1,
		CellHeight:   1,
	})
	encoder.SetOptAnyButtonPressed(len(p.mouseButtons) > 0)

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
