package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"shux/pkg/shux"
	"shux/pkg/testutil"
)

func TestDefaultLuaConfigBootstrapsSessionLifecycle(t *testing.T) {
	testutil.WithTempHome(t, func(home string) {
		configRoot := filepath.Join(home, ".config")
		configDir := filepath.Join(configRoot, "shux")
		pluginPath := filepath.Join(configDir, "lua", "plugins", "bootstrap.lua")
		configPath := filepath.Join(configDir, "init.lua")

		mustWriteFile(t, pluginPath, `
			local M = {}
			function M.setup(shux)
				shux.bind_key("k", "kill-pane")
				shux.set_session_name("lua-bootstrap")
			end
			return M
		`)
		mustWriteFile(t, configPath, `
			local shux = require("shux")
			return shux.config({
				session = { name = "config-name" },
				shell = "/bin/sh",
				mouse = false,
				keys = {
					prefix = "C-a",
					bind = {
						x = "split_vertical",
					},
				},
				plugins = {
					"plugins.bootstrap",
				},
			})
		`)

		prev := userConfigDir
		userConfigDir = func() (string, error) { return configRoot, nil }
		defer func() { userConfigDir = prev }()

		opts, err := resolveRunOptions(nil, cliOptions{})
		if err != nil {
			t.Fatalf("resolveRunOptions: %v", err)
		}
		if opts.SessionName != "lua-bootstrap" {
			t.Fatalf("session name = %q, want lua-bootstrap", opts.SessionName)
		}
		if opts.Shell != "/bin/sh" {
			t.Fatalf("shell = %q, want /bin/sh", opts.Shell)
		}
		if opts.MouseEnabled {
			t.Fatal("mouse should be disabled by config")
		}
		if got := opts.Keymap.Prefix(); got != "ctrl+a" {
			t.Fatalf("prefix = %q, want ctrl+a", got)
		}
		if binding, ok := opts.Keymap.BindingFor("x"); !ok || binding.Action != shux.ActionSplitVertical {
			t.Fatalf("expected x -> split_vertical, got %#v (ok=%t)", binding, ok)
		}
		if binding, ok := opts.Keymap.BindingFor("k"); !ok || binding.Action != shux.ActionKillPane {
			t.Fatalf("expected plugin binding k -> kill_pane, got %#v (ok=%t)", binding, ok)
		}

		super := testutil.NewTestSupervisor()
		sessionRef := shux.StartNamedSessionWithShell(1, opts.SessionName, opts.Shell, super.Handle, shux.NoOpLogger{})
		defer sessionRef.Shutdown()

		model := shux.NewModelWithOptions(sessionRef, opts.Keymap, opts.MouseEnabled)
		model = sendWindowSize(t, model, 80, 24)
		testutil.WaitSessionWindowCount(t, sessionRef, 1, time.Second)

		model = sendKey(t, model, tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
		model = sendKey(t, model, tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}))
		activeWin := <-sessionRef.Ask(shux.GetActiveWindow{})
		testutil.WaitWindowPaneCount(t, activeWin.(*shux.WindowRef), 2, time.Second)

		<-sessionRef.Ask(shux.DetachSession{})
		if !super.WaitSessionEmpty(2 * time.Second) {
			t.Fatal("timeout waiting for SessionEmpty after detach")
		}

		if !shux.SessionSnapshotExists("lua-bootstrap") {
			t.Fatal("expected snapshot for lua-bootstrap")
		}
		snapshot, err := shux.LoadSnapshot(shux.SessionSnapshotPath("lua-bootstrap"), testutil.TestLogger())
		if err != nil {
			t.Fatalf("LoadSnapshot: %v", err)
		}
		if snapshot.SessionName != "lua-bootstrap" {
			t.Fatalf("snapshot session name = %q, want lua-bootstrap", snapshot.SessionName)
		}
		if snapshot.Shell != "/bin/sh" {
			t.Fatalf("snapshot shell = %q, want /bin/sh", snapshot.Shell)
		}
		if len(snapshot.Windows) != 1 {
			t.Fatalf("snapshot windows = %d, want 1", len(snapshot.Windows))
		}
		if len(snapshot.Windows[0].PaneOrder) != 2 {
			t.Fatalf("snapshot pane count = %d, want 2", len(snapshot.Windows[0].PaneOrder))
		}

		super2 := testutil.NewTestSupervisor()
		restoredRef, err := shux.RestoreSessionFromSnapshot("lua-bootstrap", super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("RestoreSessionFromSnapshot: %v", err)
		}
		defer restoredRef.Shutdown()
		testutil.WaitSessionWindowCount(t, restoredRef, 1, time.Second)
		if restoredRef.GetSessionName() != "lua-bootstrap" {
			t.Fatalf("restored session name = %q, want lua-bootstrap", restoredRef.GetSessionName())
		}
	})
}

func TestExplicitLuaConfigPluginOverrideRestoresWorkflow(t *testing.T) {
	testutil.WithTempHome(t, func(home string) {
		configDir := filepath.Join(home, "custom-config")
		pluginPath := filepath.Join(configDir, "lua", "plugins", "workflow.lua")
		configPath := filepath.Join(configDir, "init.lua")

		mustWriteFile(t, pluginPath, `
			return {
				setup = function(shux)
					shux.set_session_name("workflow-plugin")
					shux.set_shell("/bin/sh")
					shux.bind_key("w", "split_horizontal")
				end,
			}
		`)
		mustWriteFile(t, configPath, `
			local shux = require("shux")
			shux.setup({
				options = {
					prefix = "C-g",
					session_name = "workflow-config",
				},
				plugins = {
					require("plugins.workflow"),
				},
			})
		`)

		opts, err := resolveRunOptions(nil, cliOptions{ConfigPath: configPath})
		if err != nil {
			t.Fatalf("resolveRunOptions: %v", err)
		}
		if opts.SessionName != "workflow-plugin" {
			t.Fatalf("session name = %q, want workflow-plugin", opts.SessionName)
		}
		if got := opts.Keymap.Prefix(); got != "ctrl+g" {
			t.Fatalf("prefix = %q, want ctrl+g", got)
		}

		super := testutil.NewTestSupervisor()
		sessionRef := shux.StartNamedSessionWithShell(1, opts.SessionName, opts.Shell, super.Handle, shux.NoOpLogger{})
		defer sessionRef.Shutdown()

		runner := testutil.NewScenarioRunner(sessionRef, super)
		for _, step := range testutil.FourPaneWorkflowScenario() {
			runner.AddStep(step)
		}
		runner.Run(t)

		model := shux.NewModelWithOptions(sessionRef, opts.Keymap, opts.MouseEnabled)
		model = sendWindowSize(t, model, 160, 48)
		model = sendKey(t, model, tea.KeyPressMsg(tea.Key{Code: 'g', Mod: tea.ModCtrl}))
		model = sendKey(t, model, tea.KeyPressMsg(tea.Key{Text: "w", Code: 'w'}))
		activeWindow := <-sessionRef.Ask(shux.GetActiveWindow{})
		postKeymap := testutil.WaitWindowPaneCount(t, activeWindow.(*shux.WindowRef), 5, time.Second)
		if postKeymap.ActivePane == 0 {
			t.Fatal("expected active pane after plugin-configured keybinding")
		}

		pre := <-sessionRef.Ask(shux.GetFullSessionSnapshot{})
		if pre == nil {
			t.Fatal("expected pre-detach snapshot")
		}

		<-sessionRef.Ask(shux.DetachSession{})
		if !super.WaitSessionEmpty(2 * time.Second) {
			t.Fatal("timeout waiting for SessionEmpty after detach")
		}

		super2 := testutil.NewTestSupervisor()
		restoredRef, err := shux.RestoreSessionFromSnapshot("workflow-plugin", super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("RestoreSessionFromSnapshot: %v", err)
		}
		defer restoredRef.Shutdown()
		testutil.WaitSessionWindowCount(t, restoredRef, 1, time.Second)

		post := <-restoredRef.Ask(shux.GetFullSessionSnapshot{})
		if post == nil {
			t.Fatal("expected post-restore snapshot")
		}
		testutil.AssertPersistenceInvariant(t, pre.(*shux.SessionSnapshot), post.(*shux.SessionSnapshot))
	})
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func sendWindowSize(t *testing.T, model shux.Model, width, height int) shux.Model {
	t.Helper()
	updated, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return updated.(shux.Model)
}

func sendKey(t *testing.T, model shux.Model, msg tea.KeyPressMsg) shux.Model {
	t.Helper()
	updated, _ := model.Update(msg)
	return updated.(shux.Model)
}
