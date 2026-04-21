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
	if err := runtime.installGlobalModule(L); err != nil {
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
	cfg.Keys.Prefix = strings.TrimSpace(cfg.Keys.Prefix)
	return cfg, nil
}

type luaConfigRuntime struct {
	cfg        *Config
	configPath string
	configDir  string
	plugins    []lua.LValue
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

func (r *luaConfigRuntime) installGlobalModule(L *lua.LState) error {
	mod := r.newModule(L)
	L.SetGlobal("shux", mod)

	pkg, ok := L.GetGlobal("package").(*lua.LTable)
	if !ok {
		return fmt.Errorf("lua package table unavailable")
	}
	loaded, ok := L.GetField(pkg, "loaded").(*lua.LTable)
	if !ok {
		return fmt.Errorf("lua package.loaded unavailable")
	}
	L.SetField(loaded, "shux", mod)
	return nil
}

func (r *luaConfigRuntime) runConfig(L *lua.LState) ([]lua.LValue, error) {
	baseTop := L.GetTop()
	defer L.SetTop(baseTop)

	if err := L.DoFile(r.configPath); err != nil {
		return nil, err
	}

	plugins := append([]lua.LValue(nil), r.plugins...)

	results := L.GetTop() - baseTop
	if results == 0 {
		return plugins, nil
	}
	result := L.Get(baseTop + 1)
	if result == lua.LNil {
		return plugins, nil
	}

	tbl, ok := result.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("config must return a table, got %s", result.Type().String())
	}
	resultPlugins, err := r.applyConfigTable(L, tbl)
	if err != nil {
		return nil, err
	}
	return append(plugins, resultPlugins...), nil
}

func (r *luaConfigRuntime) applyConfigTable(L *lua.LState, tbl *lua.LTable) ([]lua.LValue, error) {
	optionsValue := L.GetField(tbl, "options")
	if optionsValue != lua.LNil {
		optionsTable, ok := optionsValue.(*lua.LTable)
		if !ok {
			return nil, fmt.Errorf("options must be a table, got %s", optionsValue.Type().String())
		}
		if err := r.applyOptionsTable(L, optionsTable); err != nil {
			return nil, err
		}
	}

	keymapsValue := L.GetField(tbl, "keymaps")
	if keymapsValue != lua.LNil {
		keymapsTable, ok := keymapsValue.(*lua.LTable)
		if !ok {
			return nil, fmt.Errorf("keymaps must be a table, got %s", keymapsValue.Type().String())
		}
		if err := r.applyKeymapsTable(L, keymapsTable); err != nil {
			return nil, err
		}
	}

	if shell, ok, err := luaStringField(L, tbl, "shell"); err != nil {
		return nil, err
	} else if ok {
		r.cfg.Shell = shell
	}
	if mouse, ok, err := luaBoolField(L, tbl, "mouse"); err != nil {
		return nil, err
	} else if ok {
		r.cfg.Mouse = boolPtr(mouse)
	}

	keysValue := L.GetField(tbl, "keys")
	if keysValue != lua.LNil {
		keysTable, ok := keysValue.(*lua.LTable)
		if !ok {
			return nil, fmt.Errorf("keys must be a table, got %s", keysValue.Type().String())
		}
		if err := r.applyKeysTable(L, keysTable); err != nil {
			return nil, err
		}
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

func (r *luaConfigRuntime) applyOptionsTable(L *lua.LState, tbl *lua.LTable) error {
	var iterErr error

	tbl.ForEach(func(key, value lua.LValue) {
		if iterErr != nil {
			return
		}
		keyStr, ok := key.(lua.LString)
		if !ok {
			iterErr = fmt.Errorf("options must use string keys")
			return
		}
		name := strings.TrimSpace(string(keyStr))
		if err := r.setOptionValue(name, value); err != nil {
			iterErr = err
		}
	})
	return iterErr
}

func (r *luaConfigRuntime) applyKeymapsTable(L *lua.LState, tbl *lua.LTable) error {
	var iterErr error

	tbl.ForEach(func(key, value lua.LValue) {
		if iterErr != nil {
			return
		}
		keyStr, ok := key.(lua.LString)
		if !ok {
			iterErr = fmt.Errorf("keymaps must use string keys")
			return
		}
		tableName := strings.TrimSpace(string(keyStr))
		if tableName != "prefix" {
			iterErr = fmt.Errorf("unsupported keymap table %q", tableName)
			return
		}
		mappingTable, ok := value.(*lua.LTable)
		if !ok {
			iterErr = fmt.Errorf("keymaps.%s must be a table, got %s", tableName, value.Type().String())
			return
		}
		iterErr = r.applyPrefixKeymapTable(mappingTable)
	})
	return iterErr
}

func (r *luaConfigRuntime) applyPrefixKeymapTable(tbl *lua.LTable) error {
	var iterErr error

	tbl.ForEach(func(key, value lua.LValue) {
		if iterErr != nil {
			return
		}
		keyStr, ok := key.(lua.LString)
		if !ok {
			iterErr = fmt.Errorf("keymaps.prefix must use string keys")
			return
		}
		spec := strings.TrimSpace(string(keyStr))
		switch v := value.(type) {
		case lua.LString:
			r.cfg.Keys.SetBinding(spec, strings.TrimSpace(string(v)))
		case lua.LBool:
			if !bool(v) {
				r.cfg.Keys.AddUnbind(spec)
				return
			}
			iterErr = fmt.Errorf("keymaps.prefix[%q]: boolean true is not supported", spec)
		case *lua.LNilType:
			r.cfg.Keys.AddUnbind(spec)
		default:
			iterErr = fmt.Errorf("keymaps.prefix[%q] must be a string command or false, got %s", spec, value.Type().String())
		}
	})
	return iterErr
}

func (r *luaConfigRuntime) applyKeysTable(L *lua.LState, tbl *lua.LTable) error {
	if prefix, ok, err := luaStringField(L, tbl, "prefix"); err != nil {
		return fmt.Errorf("keys.prefix: %w", err)
	} else if ok {
		r.cfg.Keys.Prefix = prefix
	}

	bindValue := L.GetField(tbl, "bind")
	if bindValue != lua.LNil {
		bindTable, ok := bindValue.(*lua.LTable)
		if !ok {
			return fmt.Errorf("keys.bind must be a table, got %s", bindValue.Type().String())
		}
		binds, err := luaStringMap(bindTable)
		if err != nil {
			return fmt.Errorf("keys.bind: %w", err)
		}
		for spec, action := range binds {
			r.cfg.Keys.SetBinding(spec, action)
		}
	}

	unbindValue := L.GetField(tbl, "unbind")
	if unbindValue != lua.LNil {
		unbindTable, ok := unbindValue.(*lua.LTable)
		if !ok {
			return fmt.Errorf("keys.unbind must be a list, got %s", unbindValue.Type().String())
		}
		unbind, err := luaStringArray(unbindTable)
		if err != nil {
			return fmt.Errorf("keys.unbind: %w", err)
		}
		for _, spec := range unbind {
			r.cfg.Keys.AddUnbind(spec)
		}
	}

	return nil
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
	L.Push(r.newModule(L))
	return 1
}

func (r *luaConfigRuntime) newModule(L *lua.LState) *lua.LTable {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"bind_key":         r.luaBindKey,
		"config":           r.luaConfig,
		"config_dir":       r.luaConfigDir,
		"config_file":      r.luaConfigFile,
		"set_mouse":        r.luaSetMouse,
		"set_prefix":       r.luaSetPrefix,
		"set_shell":        r.luaSetShell,
		"set_session_name": r.luaSetSessionName,
		"setup":            r.luaSetup,
		"unbind_key":       r.luaUnbindKey,
	})
	L.SetField(mod, "opt", r.newOptionsTable(L))
	L.SetField(mod, "keymap", r.newKeymapTable(L))
	return mod
}

func (r *luaConfigRuntime) newOptionsTable(L *lua.LState) *lua.LTable {
	tbl := L.NewTable()
	mt := L.NewTable()
	L.SetField(mt, "__index", L.NewFunction(r.luaOptionsIndex))
	L.SetField(mt, "__newindex", L.NewFunction(r.luaOptionsNewIndex))
	L.SetMetatable(tbl, mt)
	return tbl
}

func (r *luaConfigRuntime) newKeymapTable(L *lua.LState) *lua.LTable {
	return L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"bind":   r.luaKeymapBind,
		"unbind": r.luaKeymapUnbind,
	})
}

func (r *luaConfigRuntime) luaConfig(L *lua.LState) int {
	tbl := L.CheckTable(1)
	L.Push(tbl)
	return 1
}

func (r *luaConfigRuntime) luaSetup(L *lua.LState) int {
	tbl := L.CheckTable(1)
	plugins, err := r.applyConfigTable(L, tbl)
	if err != nil {
		L.RaiseError("%s", err.Error())
		return 0
	}
	if len(plugins) > 0 {
		r.plugins = append(r.plugins, plugins...)
	}
	return 0
}

func (r *luaConfigRuntime) luaConfigDir(L *lua.LState) int {
	L.Push(lua.LString(r.configDir))
	return 1
}

func (r *luaConfigRuntime) luaConfigFile(L *lua.LState) int {
	L.Push(lua.LString(r.configPath))
	return 1
}

func (r *luaConfigRuntime) luaOptionsIndex(L *lua.LState) int {
	name := strings.TrimSpace(L.CheckString(2))
	switch name {
	case "prefix":
		if r.cfg.Keys.Prefix == "" {
			L.Push(lua.LString("C-b"))
		} else {
			L.Push(lua.LString(r.cfg.Keys.Prefix))
		}
	case "shell":
		L.Push(lua.LString(r.cfg.Shell))
	case "mouse":
		if r.cfg.Mouse == nil {
			L.Push(lua.LBool(true))
		} else {
			L.Push(lua.LBool(*r.cfg.Mouse))
		}
	case "session_name":
		L.Push(lua.LString(r.cfg.Session.Name))
	default:
		L.Push(lua.LNil)
	}
	return 1
}

func (r *luaConfigRuntime) luaOptionsNewIndex(L *lua.LState) int {
	name := strings.TrimSpace(L.CheckString(2))
	value := L.Get(3)
	if err := r.setOptionValue(name, value); err != nil {
		L.RaiseError("%s", err.Error())
	}
	return 0
}

func (r *luaConfigRuntime) luaSetMouse(L *lua.LState) int {
	enabled := L.CheckBool(1)
	r.cfg.Mouse = boolPtr(enabled)
	return 0
}

func (r *luaConfigRuntime) luaSetPrefix(L *lua.LState) int {
	r.cfg.Keys.Prefix = strings.TrimSpace(L.CheckString(1))
	return 0
}

func (r *luaConfigRuntime) luaSetShell(L *lua.LState) int {
	r.cfg.Shell = strings.TrimSpace(L.CheckString(1))
	return 0
}

func (r *luaConfigRuntime) luaSetSessionName(L *lua.LState) int {
	r.cfg.Session.Name = strings.TrimSpace(L.CheckString(1))
	return 0
}

func (r *luaConfigRuntime) luaBindKey(L *lua.LState) int {
	r.cfg.Keys.SetBinding(strings.TrimSpace(L.CheckString(1)), strings.TrimSpace(L.CheckString(2)))
	return 0
}

func (r *luaConfigRuntime) luaKeymapBind(L *lua.LState) int {
	tableName := strings.TrimSpace(L.CheckString(1))
	if tableName != "prefix" {
		L.RaiseError("unsupported keymap table %q", tableName)
		return 0
	}
	r.cfg.Keys.SetBinding(strings.TrimSpace(L.CheckString(2)), strings.TrimSpace(L.CheckString(3)))
	return 0
}

func (r *luaConfigRuntime) luaUnbindKey(L *lua.LState) int {
	r.cfg.Keys.AddUnbind(strings.TrimSpace(L.CheckString(1)))
	return 0
}

func (r *luaConfigRuntime) luaKeymapUnbind(L *lua.LState) int {
	tableName := strings.TrimSpace(L.CheckString(1))
	if tableName != "prefix" {
		L.RaiseError("unsupported keymap table %q", tableName)
		return 0
	}
	r.cfg.Keys.AddUnbind(strings.TrimSpace(L.CheckString(2)))
	return 0
}

func (r *luaConfigRuntime) setOption(name, value string) error {
	return r.setOptionValue(name, lua.LString(value))
}

func (r *luaConfigRuntime) setOptionValue(name string, value lua.LValue) error {
	switch name {
	case "prefix":
		str, ok := value.(lua.LString)
		if !ok {
			return fmt.Errorf("options.%s must be a string, got %s", name, value.Type().String())
		}
		r.cfg.Keys.Prefix = strings.TrimSpace(string(str))
		return nil
	case "shell":
		str, ok := value.(lua.LString)
		if !ok {
			return fmt.Errorf("options.%s must be a string, got %s", name, value.Type().String())
		}
		r.cfg.Shell = strings.TrimSpace(string(str))
		return nil
	case "session_name":
		str, ok := value.(lua.LString)
		if !ok {
			return fmt.Errorf("options.%s must be a string, got %s", name, value.Type().String())
		}
		r.cfg.Session.Name = strings.TrimSpace(string(str))
		return nil
	case "mouse":
		b, ok := value.(lua.LBool)
		if !ok {
			return fmt.Errorf("options.%s must be a boolean, got %s", name, value.Type().String())
		}
		r.cfg.Mouse = boolPtr(bool(b))
		return nil
	default:
		return fmt.Errorf("unknown option %q", name)
	}
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

func luaBoolField(L *lua.LState, tbl *lua.LTable, field string) (bool, bool, error) {
	value := L.GetField(tbl, field)
	if value == lua.LNil {
		return false, false, nil
	}
	b, ok := value.(lua.LBool)
	if !ok {
		return false, false, fmt.Errorf("%s must be a boolean, got %s", field, value.Type().String())
	}
	return bool(b), true, nil
}

func boolPtr(v bool) *bool {
	return &v
}

func luaStringMap(tbl *lua.LTable) (map[string]string, error) {
	result := map[string]string{}
	var iterErr error

	tbl.ForEach(func(key, value lua.LValue) {
		if iterErr != nil {
			return
		}
		keyStr, ok := key.(lua.LString)
		if !ok {
			iterErr = fmt.Errorf("must use string keys")
			return
		}
		valueStr, ok := value.(lua.LString)
		if !ok {
			iterErr = fmt.Errorf("values must be strings")
			return
		}
		result[strings.TrimSpace(string(keyStr))] = strings.TrimSpace(string(valueStr))
	})
	if iterErr != nil {
		return nil, iterErr
	}
	return result, nil
}

func luaStringArray(tbl *lua.LTable) ([]string, error) {
	values, err := luaArrayValues(tbl)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		str, ok := value.(lua.LString)
		if !ok {
			return nil, fmt.Errorf("values must be strings")
		}
		result = append(result, strings.TrimSpace(string(str)))
	}
	return result, nil
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
