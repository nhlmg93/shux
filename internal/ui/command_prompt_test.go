package ui

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"shux/internal/cfg"
	"shux/internal/protocol"
)

func TestCommandPromptOpensAndCloses(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
		Keymaps:   cfg.DefaultKeymaps(),
		MapLeader: "ctrl+b",
	})
	m = m.startCommandPrompt()
	if !m.CommandOpen || m.commandPrompt() != ":" {
		t.Fatalf("expected open command prompt, got open=%v prompt=%q", m.CommandOpen, m.commandPrompt())
	}

	updated, cmd := m.handleCommandKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if cmd != nil {
		t.Fatal("esc should not return cmd")
	}
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updated)
	}
	if got.CommandOpen {
		t.Fatal("esc should close command prompt")
	}
}

func TestCommandPromptDetachQuits(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	detached := false
	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
		OnExit: func(intent ExitIntent) {
			detached = intent == ExitDetach
		},
	})
	m.CommandInput = "detach"

	updated, cmd := m.handleCommandKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}
	got := updated.(Model)
	if got.CommandOpen || got.CommandInput != "" {
		t.Fatal("command prompt should reset after enter")
	}
	if !detached {
		t.Fatal("expected ExitDetach")
	}
}

func TestCommandPromptPrefixBinding(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
		Keymaps:   cfg.DefaultKeymaps(),
		MapLeader: "ctrl+b",
	})
	m.Prefix = true

	updated, cmd := m.handlePrefixKey(":")
	if cmd != nil {
		t.Fatal("unexpected cmd opening command prompt")
	}
	if !updated.CommandOpen {
		t.Fatal("ctrl+b : should open command prompt")
	}
}

func TestCommandHelpers(t *testing.T) {
	if got, ok := parseWindowNumber("3"); !ok || got != 3 {
		t.Fatalf("parseWindowNumber(3) = %d, %v", got, ok)
	}
	if got, ok := commandTargetIndex([]string{"-t", "2"}); !ok || got != 2 {
		t.Fatalf("commandTargetIndex -t 2 = %d, %v", got, ok)
	}
	if commandRemainder("rename-window my title", "rename-window") != "my title" {
		t.Fatal("commandRemainder should preserve spaces in name")
	}
	if commandSplitFlag([]string{"-h"}) != protocol.SplitVertical {
		t.Fatal("-h should mean left/right split")
	}
}
