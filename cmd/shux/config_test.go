package main

import (
	"os"
	"path/filepath"
	"testing"

	"shux/pkg/shux"
)

func TestDefaultConfigPath(t *testing.T) {
	prev := userConfigDir
	userConfigDir = func() (string, error) {
		return "/tmp/shux-config-home", nil
	}
	defer func() { userConfigDir = prev }()

	got, err := defaultConfigPath()
	if err != nil {
		t.Fatalf("defaultConfigPath: %v", err)
	}

	want := filepath.Join("/tmp/shux-config-home", "shux", "init.lua")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveRunOptionsUsesExplicitLuaConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "custom.lua")
	writeTestFile(t, configPath, `
		return {
			session = { name = "from-config" },
			shell = "/bin/bash",
		}
	`)

	opts, err := resolveRunOptions(nil, cliOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("resolveRunOptions: %v", err)
	}

	if opts.SessionName != "from-config" {
		t.Fatalf("expected session from config, got %q", opts.SessionName)
	}
	if opts.Shell != "/bin/bash" {
		t.Fatalf("expected shell from config, got %q", opts.Shell)
	}
}

func TestResolveRunOptionsMissingExplicitConfigFails(t *testing.T) {
	_, err := resolveRunOptions(nil, cliOptions{ConfigPath: filepath.Join(t.TempDir(), "missing.lua")})
	if err == nil {
		t.Fatal("expected explicit missing config to fail")
	}
}

func TestResolveRunOptionsUsesDefaultsWhenConfigMissing(t *testing.T) {
	prev := userConfigDir
	userConfigDir = func() (string, error) {
		return t.TempDir(), nil
	}
	defer func() { userConfigDir = prev }()

	t.Setenv("SHELL", "/bin/fish")

	opts, err := resolveRunOptions(nil, cliOptions{})
	if err != nil {
		t.Fatalf("resolveRunOptions: %v", err)
	}

	if opts.SessionName != defaultSessionName {
		t.Fatalf("expected default session %q, got %q", defaultSessionName, opts.SessionName)
	}
	if opts.Shell != "/bin/fish" {
		t.Fatalf("expected shell from SHELL env, got %q", opts.Shell)
	}
	if got := opts.Keymap.Prefix(); got != "ctrl+b" {
		t.Fatalf("expected default prefix ctrl+b, got %q", got)
	}
	if action, ok := opts.Keymap.ActionFor("c"); !ok || action != shux.ActionNewWindow {
		t.Fatalf("expected tmux default binding c -> new_window, got %q (ok=%t)", action, ok)
	}
	if action, ok := opts.Keymap.ActionFor("up"); !ok || action != shux.ActionSelectPaneUp {
		t.Fatalf("expected tmux default binding up -> select_pane_up, got %q (ok=%t)", action, ok)
	}
}

func TestResolveRunOptionsSupportsGlobalShuxModule(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "global.lua")
	writeTestFile(t, configPath, `
		shux.opt.prefix = "C-a"
		shux.keymap.bind("prefix", "h", "select-pane -L")

		shux.setup({
			options = {
				session_name = "global-config",
				shell = "/bin/bash",
			},
		})
	`)

	opts, err := resolveRunOptions(nil, cliOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("resolveRunOptions: %v", err)
	}

	if opts.SessionName != "global-config" {
		t.Fatalf("expected session from global shux config, got %q", opts.SessionName)
	}
	if opts.Shell != "/bin/bash" {
		t.Fatalf("expected shell from global shux config, got %q", opts.Shell)
	}
	if got := opts.Keymap.Prefix(); got != "ctrl+a" {
		t.Fatalf("expected prefix ctrl+a, got %q", got)
	}
	if action, ok := opts.Keymap.ActionFor("h"); !ok || action != shux.ActionSelectPaneLeft {
		t.Fatalf("expected h -> select_pane_left via global shux, got %q (ok=%t)", action, ok)
	}
}

func TestResolveRunOptionsLuaPluginModuleOverridesConfig(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "init.lua")
	pluginPath := filepath.Join(configDir, "lua", "plugins", "demo.lua")

	writeTestFile(t, pluginPath, `
		local M = {}

		function M.setup(shux)
			shux.set_shell("/bin/zsh")
			shux.set_session_name("from-plugin")
		end

		return M
	`)
	writeTestFile(t, configPath, `
		local shux = require("shux")

		return shux.config({
			session = { name = "from-config" },
			shell = "/bin/bash",
			plugins = {
				"plugins.demo",
			},
		})
	`)

	opts, err := resolveRunOptions(nil, cliOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("resolveRunOptions: %v", err)
	}

	if opts.SessionName != "from-plugin" {
		t.Fatalf("expected session override from plugin, got %q", opts.SessionName)
	}
	if opts.Shell != "/bin/zsh" {
		t.Fatalf("expected shell override from plugin, got %q", opts.Shell)
	}
}

func TestResolveRunOptionsSupportsRequiredPluginTable(t *testing.T) {
	t.Setenv("SHELL", "")

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "init.lua")
	pluginPath := filepath.Join(configDir, "lua", "plugins", "inline.lua")

	writeTestFile(t, pluginPath, `
		return {
			setup = function(shux)
				shux.set_session_name("required-plugin")
			end,
		}
	`)
	writeTestFile(t, configPath, `
		local shux = require("shux")

		return shux.config({
			plugins = {
				require("plugins.inline"),
			},
		})
	`)

	opts, err := resolveRunOptions(nil, cliOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("resolveRunOptions: %v", err)
	}

	if opts.SessionName != "required-plugin" {
		t.Fatalf("expected required plugin to set session, got %q", opts.SessionName)
	}
	if opts.Shell != shux.DefaultShell {
		t.Fatalf("expected fallback shell %q, got %q", shux.DefaultShell, opts.Shell)
	}
}

func TestResolveRunOptionsAppliesLuaKeyOverrides(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "init.lua")
	writeTestFile(t, configPath, `
		local shux = require("shux")

		return shux.config({
			keys = {
				prefix = "C-a",
				bind = {
					["h"] = "select_pane_left",
					["v"] = "split_vertical",
				},
				unbind = { "n" },
			},
		})
	`)

	opts, err := resolveRunOptions(nil, cliOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("resolveRunOptions: %v", err)
	}

	if got := opts.Keymap.Prefix(); got != "ctrl+a" {
		t.Fatalf("expected prefix ctrl+a, got %q", got)
	}
	if action, ok := opts.Keymap.ActionFor("h"); !ok || action != shux.ActionSelectPaneLeft {
		t.Fatalf("expected h override to select left pane, got %q (ok=%t)", action, ok)
	}
	if action, ok := opts.Keymap.ActionFor("v"); !ok || action != shux.ActionSplitVertical {
		t.Fatalf("expected v override to split vertically, got %q (ok=%t)", action, ok)
	}
	if _, ok := opts.Keymap.ActionFor("n"); ok {
		t.Fatal("expected n to be unbound")
	}
	if action, ok := opts.Keymap.ActionFor("c"); !ok || action != shux.ActionNewWindow {
		t.Fatalf("expected tmux default c -> new_window to remain, got %q (ok=%t)", action, ok)
	}
}

func TestResolveRunOptionsSupportsSetupKeymapsAndTmuxCommands(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "init.lua")
	writeTestFile(t, configPath, `
		shux.setup({
			options = {
				prefix = "C-a",
				session_name = "setup-style",
			},
			keymaps = {
				prefix = {
					["h"] = "select-pane -L",
					["v"] = "split-window -h",
					["n"] = false,
				},
			},
		})
	`)

	opts, err := resolveRunOptions(nil, cliOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("resolveRunOptions: %v", err)
	}

	if opts.SessionName != "setup-style" {
		t.Fatalf("expected session from setup options, got %q", opts.SessionName)
	}
	if got := opts.Keymap.Prefix(); got != "ctrl+a" {
		t.Fatalf("expected prefix ctrl+a, got %q", got)
	}
	if action, ok := opts.Keymap.ActionFor("h"); !ok || action != shux.ActionSelectPaneLeft {
		t.Fatalf("expected h -> select-pane -L, got %q (ok=%t)", action, ok)
	}
	if action, ok := opts.Keymap.ActionFor("v"); !ok || action != shux.ActionSplitVertical {
		t.Fatalf("expected v -> split-window -h, got %q (ok=%t)", action, ok)
	}
	if _, ok := opts.Keymap.ActionFor("n"); ok {
		t.Fatal("expected n to be unbound by setup keymaps")
	}
}

func TestResolveRunOptionsPluginCanModifyKeymap(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "init.lua")
	pluginPath := filepath.Join(configDir, "lua", "plugins", "keys.lua")

	writeTestFile(t, pluginPath, `
		local M = {}

		function M.setup(shux)
			shux.opt.prefix = "C-a"
			shux.keymap.bind("prefix", "h", "select-pane -L")
			shux.keymap.unbind("prefix", "n")
		end

		return M
	`)
	writeTestFile(t, configPath, `
		shux.setup({
			plugins = {
				"plugins.keys",
			},
		})
	`)

	opts, err := resolveRunOptions(nil, cliOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("resolveRunOptions: %v", err)
	}

	if got := opts.Keymap.Prefix(); got != "ctrl+a" {
		t.Fatalf("expected prefix ctrl+a, got %q", got)
	}
	if action, ok := opts.Keymap.ActionFor("h"); !ok || action != shux.ActionSelectPaneLeft {
		t.Fatalf("expected h override to select left pane, got %q (ok=%t)", action, ok)
	}
	if _, ok := opts.Keymap.ActionFor("n"); ok {
		t.Fatal("expected n to be unbound by plugin")
	}
}

func TestResolveRunOptionsRejectsUnknownKeyAction(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "init.lua")
	writeTestFile(t, configPath, `
		local shux = require("shux")

		return shux.config({
			keys = {
				bind = {
					["h"] = "warp_drive",
				},
			},
		})
	`)

	_, err := resolveRunOptions(nil, cliOptions{ConfigPath: configPath})
	if err == nil {
		t.Fatal("expected invalid action to fail")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
