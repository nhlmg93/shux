package lua_test

import (
	"os"
	"path/filepath"
	"testing"

	"shux/internal/cfg"
	"shux/internal/lua"
)

func TestLoad_defaultConfigWithoutInitLua(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))

	rt, err := lua.Load(lua.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	policy := rt.Config.WithDefaults()
	if policy.ShellPath != cfg.DefaultShellPath {
		t.Fatalf("shell = %q", policy.ShellPath)
	}
	if policy.MapLeader != cfg.DefaultMapLeader {
		t.Fatalf("mapleader = %q", policy.MapLeader)
	}
	if _, ok := policy.Keymaps.Lookup("prefix", "d"); !ok {
		t.Fatal("expected default detach binding")
	}
}

func TestLoad_userInitLuaOverridesOptions(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "shux")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))

	initLua := `shux.opt.shell = "/bin/bash"
shux.g.mapleader = "<C-a>"
shux.keymap.set("prefix", "<leader>d", "detach", { desc = "custom detach" })
`
	if err := os.WriteFile(filepath.Join(configDir, "init.lua"), []byte(initLua), 0o600); err != nil {
		t.Fatal(err)
	}

	rt, err := lua.Load(lua.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	if rt.Config.ShellPath != "/bin/bash" {
		t.Fatalf("shell = %q", rt.Config.ShellPath)
	}
	if rt.Config.MapLeader != "<C-a>" {
		t.Fatalf("mapleader = %q", rt.Config.MapLeader)
	}
	b, ok := rt.Config.Keymaps.Lookup("prefix", "d")
	if !ok || b.Desc != "custom detach" {
		t.Fatalf("binding = %#v ok=%v", b, ok)
	}
}

func TestLoad_pluginAutocmd(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "shux")
	pluginDir := filepath.Join(configDir, "plugin")
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))

	plugin := `shux.api.shux_create_autocmd({ event = "DaemonStarted", callback = function() end })`
	if err := os.WriteFile(filepath.Join(pluginDir, "demo.lua"), []byte(plugin), 0o600); err != nil {
		t.Fatal(err)
	}

	rt, err := lua.Load(lua.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
}

func TestStdpath_config(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "cfg"))
	got, err := lua.Stdpath("config")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "cfg", "shux")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
