package shux

import (
	"fmt"

	"shux/internal/lua"
)

// ReloadConfig re-reads Lua configuration from disk and applies it to the
// running daemon. Pane PTYs and session state are preserved (L3 handoff).
// A changed bind address requires a full process restart and returns an error
// so the caller can fall back to L2 spawn.
func (a *Shux) ReloadConfig(opts lua.LoadOptions) error {
	rt, err := lua.Load(opts)
	if err != nil {
		return fmt.Errorf("shux: reload config: %w", err)
	}
	newCfg := rt.Config.WithDefaults()
	currentBind := a.Config.WithDefaults().BindAddr
	if newCfg.BindAddr != currentBind {
		rt.Close()
		return fmt.Errorf("shux: bind address changed from %q to %q; full restart required", currentBind, newCfg.BindAddr)
	}
	if a.luaRuntime != nil {
		a.luaRuntime.Close()
	}
	a.Config = newCfg
	a.SetLuaRuntime(rt)
	a.SetAutocmds(rt.Autocmds)
	a.notifyClientsUIConfig()
	a.Logger.Info("shux: reloaded lua configuration")
	return nil
}
