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

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
