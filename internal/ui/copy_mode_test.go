package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"shux/internal/cfg"
	"shux/internal/luabind"
	"shux/internal/protocol"
)

type stubLuaRuntime struct {
	refs []int
}

func (s *stubLuaRuntime) CallKeymapRef(ref int) {
	s.refs = append(s.refs, ref)
}

func (s *stubLuaRuntime) Statusline(_ luabind.StatuslineContext) (string, string) {
	return "", ""
}

func (s *stubLuaRuntime) Close() {}

func TestNormalizeCopyKeyEscAndShiftG(t *testing.T) {
	esc := tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	if got := normalizeCopyKey(esc); got != "escape" {
		t.Fatalf("escape key normalized to %q", got)
	}

	shiftG := tea.KeyPressMsg(tea.Key{
		Text:        "G",
		Code:        'g',
		ShiftedCode: 'G',
		Mod:         tea.ModShift,
	})
	if got := normalizeCopyKey(shiftG); got != "shift+g" {
		t.Fatalf("shift+g normalized to %q", got)
	}
}

func TestCopyModeEscapeBindingExitsMode(t *testing.T) {
	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
	})
	m.CopyMode = true

	nextModel, _ := m.handleCopyModeKeyPress(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	next := nextModel.(Model)
	if next.CopyMode {
		t.Fatal("expected copy mode to exit on escape")
	}
}

func TestCopyModeLuaHookBindingInvokesRuntime(t *testing.T) {
	rt := &stubLuaRuntime{}
	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
		Lua:       rt,
	})
	m.CopyMode = true
	m.Keymaps = m.Keymaps.Clone()
	m.Keymaps.Set("copy_mode", "y", cfg.KeymapBinding{LuaCallback: 42, Desc: "custom yank"})

	_, _ = m.handleCopyModeKeyPress(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	if len(rt.refs) != 1 || rt.refs[0] != 42 {
		t.Fatalf("lua callback refs = %#v", rt.refs)
	}
}
