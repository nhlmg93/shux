package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"shux/pkg/shux"
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
	if err := runtime.runConfig(L); err != nil {
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

func (r *luaConfigRuntime) runConfig(L *lua.LState) error {
	baseTop := L.GetTop()
	defer L.SetTop(baseTop)

	if err := L.DoFile(r.configPath); err != nil {
		return err
	}
	if L.GetTop() != baseTop {
		return fmt.Errorf("config must not return a value; mutate the global shux runtime instead")
	}
	return nil
}

func (r *luaConfigRuntime) moduleLoader(L *lua.LState) int {
	L.Push(r.newModule(L))
	return 1
}

func (r *luaConfigRuntime) newModule(L *lua.LState) *lua.LTable {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"config_dir":    r.luaConfigDir,
		"config_file":   r.luaConfigFile,
		"list_commands": r.luaListCommands,
	})
	L.SetField(mod, "opts", r.newOptionsTable(L))
	L.SetField(mod, "keymap", r.newKeymapTable(L))
	L.SetField(mod, "cmd", r.newCommandTable(L))
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
		"set": r.luaKeymapSet,
		"del": r.luaKeymapDel,
	})
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
			L.Push(lua.LBool(false))
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

func (r *luaConfigRuntime) luaKeymapSet(L *lua.LState) int {
	tableName := strings.TrimSpace(L.CheckString(1))
	if tableName != "prefix" {
		L.RaiseError("unsupported keymap table %q", tableName)
		return 0
	}
	r.cfg.Keys.SetBinding(strings.TrimSpace(L.CheckString(2)), strings.TrimSpace(L.CheckString(3)))
	return 0
}

func (r *luaConfigRuntime) luaKeymapDel(L *lua.LState) int {
	tableName := strings.TrimSpace(L.CheckString(1))
	if tableName != "prefix" {
		L.RaiseError("unsupported keymap table %q", tableName)
		return 0
	}
	r.cfg.Keys.AddUnbind(strings.TrimSpace(L.CheckString(2)))
	return 0
}

func (r *luaConfigRuntime) setOptionValue(name string, value lua.LValue) error {
	switch name {
	case "prefix":
		str, ok := value.(lua.LString)
		if !ok {
			return fmt.Errorf("opts.%s must be a string, got %s", name, value.Type().String())
		}
		r.cfg.Keys.Prefix = strings.TrimSpace(string(str))
		return nil
	case "shell":
		str, ok := value.(lua.LString)
		if !ok {
			return fmt.Errorf("opts.%s must be a string, got %s", name, value.Type().String())
		}
		r.cfg.Shell = strings.TrimSpace(string(str))
		return nil
	case "session_name":
		str, ok := value.(lua.LString)
		if !ok {
			return fmt.Errorf("opts.%s must be a string, got %s", name, value.Type().String())
		}
		r.cfg.Session.Name = strings.TrimSpace(string(str))
		return nil
	case "mouse":
		b, ok := value.(lua.LBool)
		if !ok {
			return fmt.Errorf("opts.%s must be a boolean, got %s", name, value.Type().String())
		}
		r.cfg.Mouse = boolPtr(bool(b))
		return nil
	default:
		return fmt.Errorf("unknown option %q", name)
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func (r *luaConfigRuntime) luaListCommands(L *lua.LState) int {
	commands := shux.ValidCommands()
	tbl := L.NewTable()
	for i, name := range commands {
		L.SetTable(tbl, lua.LNumber(i+1), lua.LString(name))
	}
	L.Push(tbl)
	return 1
}

func (r *luaConfigRuntime) newCommandTable(L *lua.LState) *lua.LTable {
	commands := shux.ValidCommands()
	tbl := L.NewTable()

	for _, name := range commands {
		cmdName := name
		fn := func(L *lua.LState) int {
			var b strings.Builder
			b.WriteString(cmdName)
			for i := 1; i <= L.GetTop(); i++ {
				b.WriteString(" ")
				b.WriteString(L.CheckString(i))
			}
			cmd, err := shux.ParseCommand(b.String())
			if err != nil {
				L.Push(lua.LBool(false))
				L.Push(lua.LString(err.Error()))
				return 2
			}
			_, ok := cmd.ToActionMsg()
			L.Push(lua.LBool(ok))
			if !ok {
				L.Push(lua.LString(fmt.Sprintf("invalid arguments for %s", cmdName)))
				return 2
			}
			return 1
		}
		L.SetField(tbl, cmdName, L.NewFunction(fn))
	}

	return tbl
}
