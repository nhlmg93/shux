package shux

import (
	"testing"

	"github.com/mitchellh/go-libghostty"
)

func TestPaneEncodeKeyInputArrowModes(t *testing.T) {
	pane := newEncodingTestPane(t)

	encoded, err := pane.encodeKeyInput(KeyInput{Code: KeyCodeUp})
	if err != nil {
		t.Fatalf("encode normal up: %v", err)
	}
	if string(encoded) != "\x1b[A" {
		t.Fatalf("expected normal up arrow sequence %q, got %q", "\x1b[A", string(encoded))
	}

	if err := pane.term.ModeSet(libghostty.ModeDECCKM, true); err != nil {
		t.Fatalf("enable DECCKM: %v", err)
	}
	encoded, err = pane.encodeKeyInput(KeyInput{Code: KeyCodeUp})
	if err != nil {
		t.Fatalf("encode app up: %v", err)
	}
	if string(encoded) != "\x1bOA" {
		t.Fatalf("expected application up arrow sequence %q, got %q", "\x1bOA", string(encoded))
	}
}

func TestPaneEncodeKeyInputModifiers(t *testing.T) {
	pane := newEncodingTestPane(t)

	encoded, err := pane.encodeKeyInput(KeyInput{Code: 'a', Text: "a", Mods: KeyModAlt})
	if err != nil {
		t.Fatalf("encode alt+a: %v", err)
	}
	if string(encoded) != "\x1ba" {
		t.Fatalf("expected alt+a sequence %q, got %q", "\x1ba", string(encoded))
	}

	encoded, err = pane.encodeKeyInput(KeyInput{Code: 'c', Mods: KeyModCtrl})
	if err != nil {
		t.Fatalf("encode ctrl+c: %v", err)
	}
	if string(encoded) != "\x03" {
		t.Fatalf("expected ctrl+c sequence %q, got %q", "\x03", string(encoded))
	}
}

func TestPrefixInputEncoding(t *testing.T) {
	// Test default C-b prefix encoding
	keymap := DefaultKeymap()
	prefixInput := keymap.PrefixInput()

	pane := newEncodingTestPane(t)
	encoded, err := pane.encodeKeyInput(prefixInput)
	if err != nil {
		t.Fatalf("encode prefix input: %v", err)
	}
	// C-b should encode to byte 0x02 (STX - Start of Text)
	expected := "\x02"
	if string(encoded) != expected {
		t.Fatalf("expected prefix input to encode to %q, got %q", expected, string(encoded))
	}
}

func TestDefaultKeymapHasSendPrefixBinding(t *testing.T) {
	keymap := DefaultKeymap()

	// Verify that C-b is bound to send-prefix action
	binding, ok := keymap.BindingFor("ctrl+b")
	if !ok {
		t.Fatal("expected default keymap to have binding for ctrl+b")
	}
	if binding.Action != ActionSendPrefix {
		t.Fatalf("expected ctrl+b to bind to ActionSendPrefix, got %q", binding.Action)
	}

	// Verify prefix is still C-b
	if keymap.Prefix() != "ctrl+b" {
		t.Fatalf("expected prefix to be ctrl+b, got %q", keymap.Prefix())
	}
}

func newEncodingTestPane(t *testing.T) *Pane {
	t.Helper()

	term, err := libghostty.NewTerminal(libghostty.WithSize(80, 24))
	if err != nil {
		t.Fatalf("new terminal: %v", err)
	}
	enc, err := libghostty.NewKeyEncoder()
	if err != nil {
		term.Close()
		t.Fatalf("new key encoder: %v", err)
	}

	pane := &Pane{term: term, keyEncoder: enc}
	t.Cleanup(func() {
		enc.Close()
		term.Close()
	})
	return pane
}
