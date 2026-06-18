package shux

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"shux/internal/lua"
)

func TestReloadConfig_appliesUIChanges(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	root := t.TempDir()
	configDir := filepath.Join(root, "shux")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))

	app, err := NewShuxWithConfig(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	if app.Config.UI.Statusline {
		// default is true — good
	} else {
		t.Fatal("expected default statusline visible before reload")
	}

	initLua := `shux.opt.ui = { statusline = false, pane_border_lines = "none" }`
	if err := os.WriteFile(filepath.Join(configDir, "init.lua"), []byte(initLua), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.ReloadConfig(lua.LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	if app.Config.UI.Statusline {
		t.Fatal("expected statusline disabled after reload")
	}
	if app.Config.UI.EffectivePaneBorderLines() != "none" {
		t.Fatalf("expected pane_border_lines none after reload, got %q", app.Config.UI.EffectivePaneBorderLines())
	}
}

func TestLoad_extraSourceFile(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "shux")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))

	extra := filepath.Join(configDir, "extra.lua")
	if err := os.WriteFile(extra, []byte(`shux.g.mapleader = "<C-a>"`), 0o600); err != nil {
		t.Fatal(err)
	}
	rt, err := lua.Load(lua.LoadOptions{SourceFile: extra})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if rt.Config.MapLeader != "ctrl+a" {
		t.Fatalf("expected map_leader from sourced file, got %q", rt.Config.MapLeader)
	}
}

func TestReloadConfig_bindAddrChangeRequiresFullRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	root := t.TempDir()
	configDir := filepath.Join(root, "shux")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))

	app, err := NewShuxWithConfig(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	initLua := `shux.opt.bind = "127.0.0.1:23235"`
	if err := os.WriteFile(filepath.Join(configDir, "init.lua"), []byte(initLua), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.ReloadConfig(lua.LoadOptions{}); err == nil {
		t.Fatal("expected bind address change to fail l3 reload")
	}
}
