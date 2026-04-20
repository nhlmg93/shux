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
