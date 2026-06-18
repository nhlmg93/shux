package lua

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	glua "github.com/yuin/gopher-lua"
	"shux/internal/cfg"
	"shux/internal/luabind"
)

// Runtime holds the embedded Lua VM and frozen side effects from config load.
type Runtime struct {
	L        *glua.LState
	Config   cfg.Config
	Autocmds *cfg.AutocmdRegistry
	globals  map[string]glua.LValue
	cbTable  *glua.LTable
	cbSeq    int

	statuslineLeft  glua.LValue
	statuslineRight glua.LValue
}

// LoadOptions configures config loading.
type LoadOptions struct {
	// Bash overrides shell when spawning a new daemon (--bash flag).
	Bash bool
}

// Load reads init.lua (if present), user modules, and plugin scripts.
func Load(opts LoadOptions) (*Runtime, error) {
	policy := cfg.DefaultConfig()
	if opts.Bash {
		policy.ShellPath = cfg.BashShellPath
	}
	policy = policy.WithDefaults()

	stateDir, err := Stdpath("state")
	if err != nil {
		return nil, err
	}
	policy.StateDir = stateDir

	autocmds := cfg.NewAutocmdRegistry()
	L := glua.NewState()
	rt := &Runtime{L: L, Config: policy, Autocmds: autocmds}

	rt.installGlobals()
	if err := rt.loadUserConfig(); err != nil {
		L.Close()
		return nil, err
	}
	if err := rt.loadPlugins(); err != nil {
		L.Close()
		return nil, err
	}

	rt.Config = rt.Config.WithDefaults()
	if rt.Config.StateDir == "" {
		rt.Config.StateDir = stateDir
	}
	return rt, nil
}

func (rt *Runtime) Close() {
	if rt.L != nil {
		rt.L.Close()
		rt.L = nil
	}
}

// CallKeymapRef invokes a Lua keymap callback registered via shux.keymap.set.
func (rt *Runtime) CallKeymapRef(ref int) {
	rt.callLuaRef(ref, nil)
}

func (rt *Runtime) Statusline(ctx luabind.StatuslineContext) (left, right string) {
	left = rt.evalStatuslineSegment(rt.statuslineLeft, ctx)
	right = rt.evalStatuslineSegment(rt.statuslineRight, ctx)
	return left, right
}

func (rt *Runtime) installGlobals() {
	L := rt.L
	shuxTable := L.NewTable()
	optTable := L.NewTable()
	optMeta := L.NewTable()
	L.SetField(optMeta, "__newindex", L.NewFunction(rt.optNewIndex))
	L.SetField(optMeta, "__index", L.NewFunction(rt.optIndex))
	L.SetMetatable(optTable, optMeta)
	L.SetField(shuxTable, "opt", optTable)

	gTable := L.NewTable()
	gMeta := L.NewTable()
	L.SetField(gMeta, "__newindex", L.NewFunction(rt.gNewIndex))
	L.SetField(gMeta, "__index", L.NewFunction(rt.gIndex))
	L.SetMetatable(gTable, gMeta)
	L.SetField(shuxTable, "g", gTable)
	L.SetGlobal("shux", shuxTable)

	fnTable := L.NewTable()
	L.SetField(fnTable, "stdpath", L.NewFunction(rt.fnStdpath))
	L.SetField(shuxTable, "fn", fnTable)

	keymapTable := L.NewTable()
	L.SetField(keymapTable, "set", L.NewFunction(rt.keymapSet))
	L.SetField(shuxTable, "keymap", keymapTable)

	apiTable := L.NewTable()
	L.SetField(apiTable, "shux_get_option", L.NewFunction(rt.apiGetOption))
	L.SetField(apiTable, "shux_list_keymaps", L.NewFunction(rt.apiListKeymaps))
	L.SetField(apiTable, "shux_create_autocmd", L.NewFunction(rt.apiCreateAutocmd))
	L.SetField(apiTable, "shux_exec", L.NewFunction(rt.apiExec))
	L.SetField(apiTable, "shux_notify", L.NewFunction(rt.apiNotify))
	L.SetField(shuxTable, "api", apiTable)

	L.SetGlobal("shux", shuxTable)
	rt.globals = make(map[string]glua.LValue)
	rt.globals["mapleader"] = glua.LString(rt.Config.MapLeader)
	rt.cbTable = L.NewTable()
}

func (rt *Runtime) storeCallback(fn glua.LValue) int {
	rt.cbSeq++
	rt.cbTable.RawSetInt(rt.cbSeq, fn)
	return rt.cbSeq
}

func (rt *Runtime) gIndex(L *glua.LState) int {
	key := L.CheckString(2)
	if v, ok := rt.globals[key]; ok {
		L.Push(v)
		return 1
	}
	L.Push(glua.LNil)
	return 1
}

func (rt *Runtime) gNewIndex(L *glua.LState) int {
	key := L.CheckString(2)
	val := L.Get(3)
	rt.globals[key] = val
	if key == "mapleader" {
		if s, ok := val.(glua.LString); ok {
			rt.Config.MapLeader = cfg.NormalizeMapLeader(string(s))
		}
	}
	return 0
}

func (rt *Runtime) optIndex(L *glua.LState) int {
	key := L.CheckString(2)
	switch key {
	case "shell":
		L.Push(glua.LString(rt.Config.ShellPath))
	case "bind":
		L.Push(glua.LString(rt.Config.BindAddr))
	case "scrollback":
		L.Push(glua.LNumber(rt.Config.Scrollback))
	case "max_sessions":
		L.Push(glua.LNumber(rt.Config.MaxSessions))
	case "journal_max_mb":
		L.Push(glua.LNumber(rt.Config.JournalMaxMB))
	case "journal_replay_delay_ms":
		L.Push(glua.LNumber(rt.Config.JournalReplayDelay.Milliseconds()))
	case "display_panes_timeout_ms":
		L.Push(glua.LNumber(rt.Config.PaneQuickSelectTimeout.Milliseconds()))
	case "state_dir":
		L.Push(glua.LString(rt.Config.StateDir))
	case "resurrection":
		L.Push(glua.LBool(rt.Config.Resurrection))
	case "statusline":
		L.Push(rt.statuslineTable())
	case "ui":
		L.Push(rt.uiTable())
	default:
		L.Push(glua.LNil)
	}
	return 1
}

func (rt *Runtime) optNewIndex(L *glua.LState) int {
	key := L.CheckString(2)
	val := L.Get(3)
	switch key {
	case "shell":
		rt.Config.ShellPath = luaString(val)
	case "bind":
		rt.Config.BindAddr = luaString(val)
	case "scrollback":
		rt.Config.Scrollback = uint(luaNumber(val))
	case "max_sessions":
		rt.Config.MaxSessions = uint(luaNumber(val))
	case "journal_max_mb":
		rt.Config.JournalMaxMB = uint(luaNumber(val))
	case "journal_replay_delay_ms":
		rt.Config.JournalReplayDelay = time.Duration(luaNumber(val)) * time.Millisecond
	case "display_panes_timeout_ms":
		rt.Config.PaneQuickSelectTimeout = time.Duration(luaNumber(val)) * time.Millisecond
	case "state_dir":
		rt.Config.StateDir = luaString(val)
	case "resurrection":
		rt.Config.Resurrection = luaBool(val)
	case "statusline":
		rt.statuslineFromLua(val)
	case "ui":
		rt.uiFromLua(val)
	}
	return 0
}

func (rt *Runtime) statuslineFromLua(v glua.LValue) {
	rt.statuslineLeft = glua.LNil
	rt.statuslineRight = glua.LNil
	tbl, ok := v.(*glua.LTable)
	if !ok {
		return
	}
	rt.statuslineLeft = tbl.RawGetString("left")
	rt.statuslineRight = tbl.RawGetString("right")
}

func (rt *Runtime) uiFromLua(v glua.LValue) {
	tbl, ok := v.(*glua.LTable)
	if !ok {
		return
	}
	u := rt.Config.UI.WithDefaults()
	if val := tbl.RawGetString("statusline"); val != glua.LNil {
		u.Statusline = luaBool(val)
	}
	// Deprecated: prefer pane_border_lines.
	if val := tbl.RawGetString("pane_borders"); val != glua.LNil {
		u.PaneBorders = luaBool(val)
	}
	if val := tbl.RawGetString("pane_border_lines"); val != glua.LNil {
		if mode := cfg.NormalizePaneBorderLines(luaString(val)); mode != "" {
			u.PaneBorderLines = mode
		}
	}
	if val := tbl.RawGetString("pane_outer_border"); val != glua.LNil {
		u.PaneOuterBorder = luaBool(val)
	}
	if val := tbl.RawGetString("pane_border_style"); val != glua.LNil {
		u.PaneBorderStyle = luaString(val)
	}
	if val := tbl.RawGetString("pane_active_border_style"); val != glua.LNil {
		u.PaneActiveBorderStyle = luaString(val)
	}
	if val := tbl.RawGetString("pane_labels"); val != glua.LNil {
		u.PaneLabels = luaBool(val)
	}
	if val := tbl.RawGetString("statusline_style"); val != glua.LNil {
		u.StatuslineStyle = luaString(val)
	}
	if val := tbl.RawGetString("search_match_ansi"); val != glua.LNil {
		u.SearchMatchANSI = luaString(val)
	}
	if val := tbl.RawGetString("search_active_ansi"); val != glua.LNil {
		u.SearchActiveANSI = luaString(val)
	}
	if val := tbl.RawGetString("copy_mode_status_ansi"); val != glua.LNil {
		u.CopyModeStatusANSI = luaString(val)
	}
	if tbl.RawGetString("pane_border_lines") == glua.LNil {
		u.PaneBorderLines = ""
		if u.PaneBorders {
			u.PaneBorderLines = cfg.PaneBorderLinesSingle
		} else {
			u.PaneBorderLines = cfg.PaneBorderLinesNone
		}
	}
	rt.Config.UI = u.WithDefaults()
}

func (rt *Runtime) uiTable() *glua.LTable {
	u := rt.Config.UI.WithDefaults()
	tbl := rt.L.NewTable()
	rt.L.SetField(tbl, "statusline", glua.LBool(u.Statusline))
	rt.L.SetField(tbl, "pane_borders", glua.LBool(u.PaneBorders))
	rt.L.SetField(tbl, "pane_border_lines", glua.LString(u.EffectivePaneBorderLines()))
	rt.L.SetField(tbl, "pane_outer_border", glua.LBool(u.PaneOuterBorder))
	if u.PaneBorderStyle != "" {
		rt.L.SetField(tbl, "pane_border_style", glua.LString(u.PaneBorderStyle))
	}
	if u.PaneActiveBorderStyle != "" {
		rt.L.SetField(tbl, "pane_active_border_style", glua.LString(u.PaneActiveBorderStyle))
	}
	rt.L.SetField(tbl, "pane_labels", glua.LBool(u.PaneLabels))
	rt.L.SetField(tbl, "statusline_style", glua.LString(u.StatuslineStyle))
	if u.SearchMatchANSI != "" {
		rt.L.SetField(tbl, "search_match_ansi", glua.LString(u.SearchMatchANSI))
	}
	if u.SearchActiveANSI != "" {
		rt.L.SetField(tbl, "search_active_ansi", glua.LString(u.SearchActiveANSI))
	}
	if u.CopyModeStatusANSI != "" {
		rt.L.SetField(tbl, "copy_mode_status_ansi", glua.LString(u.CopyModeStatusANSI))
	}
	return tbl
}

func (rt *Runtime) statuslineTable() *glua.LTable {
	tbl := rt.L.NewTable()
	if rt.statuslineLeft != nil && rt.statuslineLeft != glua.LNil {
		rt.L.SetField(tbl, "left", rt.statuslineLeft)
	}
	if rt.statuslineRight != nil && rt.statuslineRight != glua.LNil {
		rt.L.SetField(tbl, "right", rt.statuslineRight)
	}
	return tbl
}

func (rt *Runtime) evalStatuslineSegment(segment glua.LValue, ctx luabind.StatuslineContext) string {
	switch v := segment.(type) {
	case glua.LString:
		return strings.TrimSpace(string(v))
	case *glua.LTable:
		return strings.TrimSpace(rt.evalStatuslineList(v, ctx))
	case *glua.LFunction:
		L := rt.L
		ctxTable := L.NewTable()
		L.SetField(ctxTable, "session_id", glua.LString(ctx.SessionID))
		L.SetField(ctxTable, "window_id", glua.LString(ctx.WindowID))
		L.SetField(ctxTable, "window_index", glua.LNumber(ctx.WindowIndex))
		L.SetField(ctxTable, "window_name", glua.LString(ctx.WindowName))
		L.SetField(ctxTable, "active_pane", glua.LString(ctx.ActivePane))
		L.SetField(ctxTable, "hostname", glua.LString(ctx.Hostname))
		L.SetField(ctxTable, "title", glua.LString(ctx.Title))
		L.SetField(ctxTable, "status", glua.LString(ctx.Status))
		if err := L.CallByParam(glua.P{Fn: v, NRet: 1, Protect: true}, ctxTable); err != nil {
			fmt.Fprintf(os.Stderr, "shux: statusline callback: %v\n", err)
			return ""
		}
		out := L.Get(-1)
		L.Pop(1)
		if out == glua.LNil {
			return ""
		}
		return strings.TrimSpace(out.String())
	default:
		return ""
	}
}

func (rt *Runtime) evalStatuslineList(tbl *glua.LTable, ctx luabind.StatuslineContext) string {
	parts := make([]string, 0, tbl.Len())
	for i := 1; i <= tbl.Len(); i++ {
		part := strings.TrimSpace(rt.evalStatuslineSegment(tbl.RawGetInt(i), ctx))
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " ")
}

func (rt *Runtime) fnStdpath(L *glua.LState) int {
	name := L.CheckString(1)
	path, err := Stdpath(name)
	if err != nil {
		L.RaiseError("stdpath(%q): %s", name, err)
		return 0
	}
	L.Push(glua.LString(path))
	return 1
}

func (rt *Runtime) keymapSet(L *glua.LState) int {
	mode := L.CheckString(1)
	lhs := L.CheckString(2)
	rhs := L.Get(3)
	desc := ""
	if L.GetTop() >= 4 {
		opts := L.CheckTable(4)
		if d := opts.RawGetString("desc"); d != glua.LNil {
			desc = luaString(d)
		}
	}

	key := cfg.ExpandLeaderKey(lhs)
	key = strings.ToLower(strings.TrimSpace(key))

	b := cfg.KeymapBinding{Desc: desc}
	switch rhs.Type() {
	case glua.LTString:
		b.Builtin = cfg.BuiltinKeyAction(luaString(rhs))
	case glua.LTFunction:
		b.LuaCallback = rt.storeCallback(rhs)
	default:
		L.RaiseError("keymap rhs must be string action or function")
		return 0
	}
	rt.Config.Keymaps.Set(mode, key, b)
	return 0
}

func (rt *Runtime) apiGetOption(L *glua.LState) int {
	name := L.CheckString(1)
	switch name {
	case "shell":
		L.Push(glua.LString(rt.Config.ShellPath))
	case "bind":
		L.Push(glua.LString(rt.Config.BindAddr))
	case "scrollback":
		L.Push(glua.LNumber(rt.Config.Scrollback))
	case "max_sessions":
		L.Push(glua.LNumber(rt.Config.MaxSessions))
	case "journal_max_mb":
		L.Push(glua.LNumber(rt.Config.JournalMaxMB))
	case "journal_replay_delay_ms":
		L.Push(glua.LNumber(rt.Config.JournalReplayDelay.Milliseconds()))
	case "display_panes_timeout_ms":
		L.Push(glua.LNumber(rt.Config.PaneQuickSelectTimeout.Milliseconds()))
	case "state_dir":
		L.Push(glua.LString(rt.Config.StateDir))
	case "resurrection":
		L.Push(glua.LBool(rt.Config.Resurrection))
	case "statusline":
		L.Push(rt.statuslineTable())
	case "ui":
		L.Push(rt.uiTable())
	default:
		L.Push(glua.LNil)
	}
	return 1
}

func (rt *Runtime) apiListKeymaps(L *glua.LState) int {
	mode := "prefix"
	if L.GetTop() >= 1 && L.Get(1) != glua.LNil {
		mode = L.CheckString(1)
	}
	tbl := L.NewTable()
	i := 1
	for _, b := range rt.Config.Keymaps.List(mode) {
		row := L.NewTable()
		L.SetField(row, "key", glua.LString(b.Key))
		L.SetField(row, "desc", glua.LString(b.Desc))
		if b.Builtin != "" {
			L.SetField(row, "action", glua.LString(string(b.Builtin)))
		}
		tbl.RawSetInt(i, row)
		i++
	}
	L.Push(tbl)
	return 1
}

func (rt *Runtime) apiCreateAutocmd(L *glua.LState) int {
	opts := L.CheckTable(1)
	eventVal := opts.RawGet(glua.LString("event"))
	callbackVal := opts.RawGet(glua.LString("callback"))
	if eventVal == glua.LNil || callbackVal == glua.LNil {
		L.RaiseError("shux_create_autocmd requires event and callback")
		return 0
	}
	event := cfg.AutocmdEvent(luaString(eventVal))
	if callbackVal.Type() != glua.LTFunction {
		L.RaiseError("callback must be a function")
		return 0
	}
	ref := rt.storeCallback(callbackVal)
	rt.Autocmds.Subscribe(event, func(_ context.Context, data map[string]any) {
		rt.callLuaRef(ref, data)
	})
	return 0
}

func (rt *Runtime) callLuaRef(ref int, data map[string]any) {
	L := rt.L
	if L == nil || ref == 0 || rt.cbTable == nil {
		return
	}
	fn := rt.cbTable.RawGetInt(ref)
	if fn == glua.LNil || fn.Type() != glua.LTFunction {
		return
	}
	L.Push(fn)
	if data != nil {
		tbl := L.NewTable()
		for k, v := range data {
			L.SetField(tbl, k, goToLua(L, v))
		}
		if err := L.CallByParam(glua.P{Fn: fn, NRet: 0, Protect: true}, tbl); err != nil {
			fmt.Fprintf(os.Stderr, "shux: lua callback: %v\n", err)
		}
		return
	}
	if err := L.CallByParam(glua.P{Fn: fn, NRet: 0, Protect: true}); err != nil {
		fmt.Fprintf(os.Stderr, "shux: lua callback: %v\n", err)
	}
}

func (rt *Runtime) apiExec(L *glua.LState) int {
	// Stored for plugins; UI/daemon dispatch happens via registered exec hooks post-MVP.
	_ = L.CheckString(1)
	return 0
}

func (rt *Runtime) apiNotify(L *glua.LState) int {
	msg := L.CheckString(1)
	fmt.Fprintf(os.Stderr, "shux: %s\n", msg)
	return 0
}

func (rt *Runtime) loadUserConfig() error {
	configDir, err := Stdpath("config")
	if err != nil {
		return err
	}
	initPath := filepath.Join(configDir, "init.lua")
	if _, err := os.Stat(initPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := rt.prependLoader(configDir); err != nil {
		return err
	}
	return rt.doFile(initPath)
}

func (rt *Runtime) prependLoader(configDir string) error {
	luaDir := filepath.Join(configDir, "lua")
	searcher := func(L *glua.LState) int {
		name := L.CheckString(1)
		rel := strings.ReplaceAll(name, ".", string(filepath.Separator))
		candidates := []string{
			filepath.Join(luaDir, rel+".lua"),
			filepath.Join(luaDir, rel, "init.lua"),
		}
		for _, path := range candidates {
			if _, err := os.Stat(path); err != nil {
				continue
			}
			if err := rt.doFile(path); err != nil {
				L.RaiseError("require %q: %s", name, err)
				return 0
			}
			L.Push(glua.LTrue)
			return 1
		}
		L.Push(glua.LString(fmt.Sprintf("module %q not found under %s", name, luaDir)))
		return 1
	}
	L := rt.L
	loaders, ok := L.GetField(L.Get(glua.RegistryIndex), "_LOADERS").(*glua.LTable)
	if !ok {
		return nil
	}
	loaders.Insert(2, L.NewFunction(searcher))
	return nil
}

func (rt *Runtime) loadPlugins() error {
	if err := rt.loadPluginDirFromFS(embeddedRuntimeDir("plugin"), "."); err != nil {
		return err
	}
	configDir, err := Stdpath("config")
	if err != nil {
		return err
	}
	userPlugin := filepath.Join(configDir, "plugin")
	if err := rt.loadPluginDirFromOS(userPlugin); err != nil {
		return err
	}
	return rt.loadPackPlugins(configDir)
}

func (rt *Runtime) loadPluginDirFromFS(root fs.FS, dir string) error {
	files, err := listLuaFiles(root, dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, rel := range files {
		src, err := fs.ReadFile(root, rel)
		if err != nil {
			return err
		}
		if err := rt.doString(string(src), rel); err != nil {
			return fmt.Errorf("plugin %s: %w", rel, err)
		}
	}
	return nil
}

func (rt *Runtime) loadPluginDirFromOS(dir string) error {
	files, err := listLuaFiles(os.DirFS(dir), ".")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, rel := range files {
		path := filepath.Join(dir, rel)
		if err := rt.doFile(path); err != nil {
			return fmt.Errorf("plugin %s: %w", path, err)
		}
	}
	return nil
}

func (rt *Runtime) loadPackPlugins(configDir string) error {
	dataDir, err := Stdpath("data")
	if err != nil {
		return err
	}
	for _, base := range []string{
		filepath.Join(configDir, "pack"),
		filepath.Join(dataDir, "site", "pack"),
	} {
		if err := rt.walkPack(base); err != nil {
			return err
		}
	}
	return nil
}

func (rt *Runtime) walkPack(packRoot string) error {
	info, err := os.Stat(packRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(packRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".lua" {
			return nil
		}
		slash := filepath.ToSlash(path)
		if !strings.Contains(slash, "/start/") {
			return nil
		}
		if !strings.Contains(slash, "/plugin/") {
			return nil
		}
		return rt.doFile(path)
	})
}

func (rt *Runtime) doFile(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return rt.doString(string(src), path)
}

func (rt *Runtime) doString(src, name string) error {
	fn, err := rt.L.LoadString(src)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	rt.L.Push(fn)
	return rt.L.PCall(0, glua.MultRet, nil)
}

func luaString(v glua.LValue) string {
	if v == glua.LNil {
		return ""
	}
	return v.String()
}

func luaNumber(v glua.LValue) float64 {
	if n, ok := v.(glua.LNumber); ok {
		return float64(n)
	}
	return 0
}

func luaBool(v glua.LValue) bool {
	if b, ok := v.(glua.LBool); ok {
		return bool(b)
	}
	return false
}

func goToLua(L *glua.LState, v any) glua.LValue {
	switch x := v.(type) {
	case string:
		return glua.LString(x)
	case bool:
		return glua.LBool(x)
	case int:
		return glua.LNumber(x)
	case float64:
		return glua.LNumber(x)
	default:
		return glua.LNil
	}
}
