package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

func loadConfig(path string, explicit bool) (Config, error) {
	cfg := Config{}

	if path == "" {
		return cfg, nil
	}
	if _, err := os.Stat(path); err != nil {
		if !explicit && errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}

	L := lua.NewState()
	defer L.Close()

	runtime := &luaConfigRuntime{
		cfg:        &cfg,
		configPath: path,
		configDir:  filepath.Dir(path),
	}
	L.PreloadModule("shux", runtime.moduleLoader)

	if err := runtime.configurePackagePath(L); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}

	plugins, err := runtime.runConfig(L)
	if err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}
	if err := runtime.runPlugins(L, plugins); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}

	cfg.Session.Name = strings.TrimSpace(cfg.Session.Name)
	cfg.Shell = strings.TrimSpace(cfg.Shell)
	return cfg, nil
}

type luaConfigRuntime struct {
	cfg        *Config
	configPath string
	configDir  string
}

func (r *luaConfigRuntime) configurePackagePath(L *lua.LState) error {
	pkg, ok := L.GetGlobal("package").(*lua.LTable)
	if !ok {
		return fmt.Errorf("lua package table unavailable")
	}

	paths := []string{
		filepath.ToSlash(filepath.Join(r.configDir, "?.lua")),
		filepath.ToSlash(filepath.Join(r.configDir, "?", "init.lua")),
		filepath.ToSlash(filepath.Join(r.configDir, "lua", "?.lua")),
		filepath.ToSlash(filepath.Join(r.configDir, "lua", "?", "init.lua")),
	}
	current := lua.LVAsString(L.GetField(pkg, "path"))
	if current != "" {
		paths = append(paths, current)
	}
	L.SetField(pkg, "path", lua.LString(strings.Join(paths, ";")))
	return nil
}

func (r *luaConfigRuntime) runConfig(L *lua.LState) ([]lua.LValue, error) {
	baseTop := L.GetTop()
	defer L.SetTop(baseTop)

	if err := L.DoFile(r.configPath); err != nil {
		return nil, err
	}

	results := L.GetTop() - baseTop
	if results == 0 {
		return nil, nil
	}
	result := L.Get(baseTop + 1)
	if result == lua.LNil {
		return nil, nil
	}

	tbl, ok := result.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("config must return a table, got %s", result.Type().String())
	}
	return r.applyConfigTable(L, tbl)
}

func (r *luaConfigRuntime) applyConfigTable(L *lua.LState, tbl *lua.LTable) ([]lua.LValue, error) {
	if shell, ok, err := luaStringField(L, tbl, "shell"); err != nil {
		return nil, err
	} else if ok {
		r.cfg.Shell = shell
	}

	sessionValue := L.GetField(tbl, "session")
	if sessionValue != lua.LNil {
		sessionTable, ok := sessionValue.(*lua.LTable)
		if !ok {
			return nil, fmt.Errorf("session must be a table, got %s", sessionValue.Type().String())
		}
		if sessionName, ok, err := luaStringField(L, sessionTable, "name"); err != nil {
			return nil, err
		} else if ok {
			r.cfg.Session.Name = sessionName
		}
	}

	pluginsValue := L.GetField(tbl, "plugins")
	if pluginsValue == lua.LNil {
		return nil, nil
	}
	pluginsTable, ok := pluginsValue.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("plugins must be a list, got %s", pluginsValue.Type().String())
	}

	plugins, err := luaArrayValues(pluginsTable)
	if err != nil {
		return nil, fmt.Errorf("plugins: %w", err)
	}
	return plugins, nil
}

func (r *luaConfigRuntime) runPlugins(L *lua.LState, plugins []lua.LValue) error {
	for i, plugin := range plugins {
		if err := r.runPlugin(L, plugin); err != nil {
			return fmt.Errorf("plugin %d: %w", i+1, err)
		}
	}
	return nil
}

func (r *luaConfigRuntime) runPlugin(L *lua.LState, spec lua.LValue) error {
	plugin := spec
	if moduleName, ok := spec.(lua.LString); ok {
		loaded, err := requireModule(L, string(moduleName))
		if err != nil {
			return fmt.Errorf("require %q: %w", string(moduleName), err)
		}
		plugin = loaded
	}

	switch value := plugin.(type) {
	case *lua.LFunction:
		return r.callSetup(L, value)
	case *lua.LTable:
		setup := L.GetField(value, "setup")
		if setup == lua.LNil {
			return nil
		}
		fn, ok := setup.(*lua.LFunction)
		if !ok {
			return fmt.Errorf("plugin setup must be a function, got %s", setup.Type().String())
		}
		return r.callSetup(L, fn)
	default:
		return fmt.Errorf("plugin must be a module name, function, or table, got %s", plugin.Type().String())
	}
}

func (r *luaConfigRuntime) callSetup(L *lua.LState, fn *lua.LFunction) error {
	mod, err := requireModule(L, "shux")
	if err != nil {
		return err
	}
	return L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, mod)
}

func requireModule(L *lua.LState, name string) (lua.LValue, error) {
	require, ok := L.GetGlobal("require").(*lua.LFunction)
	if !ok {
		return lua.LNil, fmt.Errorf("lua require() unavailable")
	}
	if err := L.CallByParam(lua.P{Fn: require, NRet: 1, Protect: true}, lua.LString(name)); err != nil {
		return lua.LNil, err
	}
	result := L.Get(-1)
	L.Pop(1)
	return result, nil
}

func (r *luaConfigRuntime) moduleLoader(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"config":           r.luaConfig,
		"config_dir":       r.luaConfigDir,
		"config_file":      r.luaConfigFile,
		"set_shell":        r.luaSetShell,
		"set_session_name": r.luaSetSessionName,
	})
	L.Push(mod)
	return 1
}

func (r *luaConfigRuntime) luaConfig(L *lua.LState) int {
	tbl := L.CheckTable(1)
	L.Push(tbl)
	return 1
}

func (r *luaConfigRuntime) luaConfigDir(L *lua.LState) int {
	L.Push(lua.LString(r.configDir))
	return 1
}

func (r *luaConfigRuntime) luaConfigFile(L *lua.LState) int {
	L.Push(lua.LString(r.configPath))
	return 1
}

func (r *luaConfigRuntime) luaSetShell(L *lua.LState) int {
	r.cfg.Shell = strings.TrimSpace(L.CheckString(1))
	return 0
}

func (r *luaConfigRuntime) luaSetSessionName(L *lua.LState) int {
	r.cfg.Session.Name = strings.TrimSpace(L.CheckString(1))
	return 0
}

func luaStringField(L *lua.LState, tbl *lua.LTable, field string) (string, bool, error) {
	value := L.GetField(tbl, field)
	if value == lua.LNil {
		return "", false, nil
	}
	str, ok := value.(lua.LString)
	if !ok {
		return "", false, fmt.Errorf("%s must be a string, got %s", field, value.Type().String())
	}
	return strings.TrimSpace(string(str)), true, nil
}

func luaArrayValues(tbl *lua.LTable) ([]lua.LValue, error) {
	valuesByIndex := make(map[int]lua.LValue)
	maxIndex := 0
	count := 0
	var iterErr error

	tbl.ForEach(func(key, value lua.LValue) {
		if iterErr != nil {
			return
		}
		index, ok := key.(lua.LNumber)
		if !ok || index < 1 || lua.LNumber(int(index)) != index {
			iterErr = fmt.Errorf("must be a dense list")
			return
		}
		idx := int(index)
		valuesByIndex[idx] = value
		if idx > maxIndex {
			maxIndex = idx
		}
		count++
	})
	if iterErr != nil {
		return nil, iterErr
	}
	if count == 0 {
		return nil, nil
	}

	values := make([]lua.LValue, 0, maxIndex)
	for i := 1; i <= maxIndex; i++ {
		value, ok := valuesByIndex[i]
		if !ok || value == lua.LNil {
			return nil, fmt.Errorf("must be a dense list")
		}
		values = append(values, value)
	}
	return values, nil
}
