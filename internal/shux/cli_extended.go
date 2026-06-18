package shux

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"shux/internal/cfg"
	"shux/internal/lua"
	"shux/internal/protocol"
	"shux/internal/ui"
)

func (a *Shux) handleExtendedCLI(ctx context.Context, name string, args []string, out io.Writer) (bool, error) {
	switch name {
	case "kill-server":
		return true, a.cliKillServer()
	case "source-file", "source":
		return true, a.cliSourceFile(ctx, args, out)
	case "list-clients", "lsc":
		return true, a.cliListClients(out)
	case "switch-client", "switchc":
		return true, a.cliSwitchClient(ctx, args)
	case "show-options", "show":
		return true, a.cliShowOptions(args, out)
	case "set-option", "set":
		return true, a.cliSetOption(ctx, args)
	case "show-environment", "showenv":
		return true, a.cliShowEnvironment(ctx, args, out)
	case "set-environment", "setenv":
		return true, a.cliSetEnvironment(ctx, args)
	case "list-keys", "lsk":
		return true, a.cliListKeys(out)
	case "bind-key", "bind":
		return true, a.cliBindKey(args)
	case "list-buffers", "lsb":
		return true, a.cliListBuffers(out)
	case "set-buffer", "setb":
		return true, a.cliSetBuffer(args)
	case "paste-buffer", "pasteb":
		return true, a.cliPasteBuffer(ctx, args)
	case "resize-pane", "resizep":
		return true, a.cliResizePane(ctx, args)
	case "swap-pane", "swapp":
		return true, a.cliSwapPane(ctx, args)
	case "move-pane", "movep":
		return true, a.cliMovePane(ctx, args)
	case "break-pane", "breakp":
		return true, a.cliBreakPane(ctx, args)
	case "join-pane", "joinp":
		return true, a.cliJoinPane(ctx, args)
	case "select-layout", "selectl":
		return true, a.cliSelectLayout(ctx, args)
	case "choose-tree":
		return true, a.cliChooseTree(args)
	case "command-prompt":
		return true, a.cliCommandPrompt(args)
	case "display-menu", "menu":
		return true, a.cliDisplayMenu(out)
	default:
		return false, nil
	}
}

func (a *Shux) cliKillServer() error {
	a.DetachAllClients()
	a.RequestShutdown()
	return nil
}

func (a *Shux) cliSourceFile(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("shux: source-file requires PATH")
	}
	path := args[len(args)-1]
	if !filepath.IsAbs(path) {
		if home, err := os.UserHomeDir(); err == nil {
			candidate := filepath.Join(home, ".config", "shux", path)
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			}
		}
	}
	opts := lua.LoadOptions{Bash: a.Config.ShellPath == cfg.BashShellPath}
	if err := a.ReloadConfig(opts); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "sourced %s\n", path)
	return err
}

func (a *Shux) cliListClients(out io.Writer) error {
	for _, c := range a.clientReg.List() {
		marker := " "
		if c.SessionID == a.DefaultSessionID {
			marker = "*"
		}
		if _, err := fmt.Fprintf(out, "%s %s\t%s\n", marker, c.ClientID, c.SessionID); err != nil {
			return err
		}
	}
	return nil
}

func (a *Shux) cliSwitchClient(ctx context.Context, args []string) error {
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	sessionName := targetSpec
	if sessionName == "" && len(rest) > 0 {
		sessionName = rest[0]
	}
	if sessionName == "" {
		return fmt.Errorf("shux: switch-client requires -t SESSION")
	}
	sess, err := a.ResolveSession(ctx, sessionName)
	if err != nil {
		return err
	}
	a.DefaultSessionID = sess.SessionID
	a.DefaultSession = sess.Name
	windowIDs := a.cache.WindowIDs(sess.SessionID)
	if len(windowIDs) > 0 {
		a.DefaultWindowID = windowIDs[0]
		if layout, ok := a.cache.LayoutSnapshot(sess.SessionID, windowIDs[0]); ok && len(layout.Panes) > 0 {
			a.DefaultPaneID = layout.Panes[0].PaneID
		}
	}
	if id, ok := a.clientReg.First(); ok {
		a.clientReg.SetSession(id, sess.SessionID)
		a.sendClientUI(id, ui.ClientUIMsg{Action: ui.ClientUIActionSwitchSession, SessionID: sess.SessionID})
	}
	return nil
}

func (a *Shux) cliShowOptions(args []string, out io.Writer) error {
	c := a.Config.WithDefaults()
	opts := map[string]string{
		"shell-path":                c.ShellPath,
		"bind-address":              c.BindAddr,
		"map-leader":                c.MapLeader,
		"scrollback":                strconv.FormatUint(uint64(c.Scrollback), 10),
		"max-sessions":              strconv.FormatUint(uint64(c.MaxSessions), 10),
		"journal-max-mb":            strconv.FormatUint(uint64(c.JournalMaxMB), 10),
		"resurrection":              strconv.FormatBool(c.Resurrection),
		"pane-quick-select-timeout": c.PaneQuickSelectTimeout.String(),
	}
	key := ""
	if len(args) > 0 {
		key = args[len(args)-1]
	}
	for k, v := range opts {
		if key != "" && k != key {
			continue
		}
		if _, err := fmt.Fprintf(out, "%s %s\n", k, v); err != nil {
			return err
		}
	}
	return nil
}

func (a *Shux) cliSetOption(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("shux: set-option requires OPTION VALUE")
	}
	key, value := args[0], args[1]
	switch key {
	case "statusline":
		uiCfg := a.Config.UI.WithDefaults()
		uiCfg.Statusline = value == "on" || value == "true" || value == "1"
		a.Config.UI = uiCfg
		a.notifyClientsUIConfig()
		return nil
	default:
		return fmt.Errorf("shux: set-option %q requires daemon reload via source-file", key)
	}
}

func (a *Shux) cliShowEnvironment(ctx context.Context, args []string, out io.Writer) error {
	sid := a.DefaultSessionID
	if len(args) > 0 {
		sess, err := a.ResolveSession(ctx, args[0])
		if err != nil {
			return err
		}
		sid = sess.SessionID
	}
	for k, v := range a.sessionEnv.List(sid) {
		if _, err := fmt.Fprintf(out, "%s=%s\n", k, v); err != nil {
			return err
		}
	}
	return nil
}

func (a *Shux) cliSetEnvironment(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("shux: set-environment requires -t SESSION VAR VALUE")
	}
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	if len(rest) < 2 {
		return fmt.Errorf("shux: set-environment requires VAR VALUE")
	}
	sid := a.DefaultSessionID
	if targetSpec != "" {
		sess, err := a.ResolveSession(ctx, targetSpec)
		if err != nil {
			return err
		}
		sid = sess.SessionID
	}
	if rest[0] == "-u" || rest[0] == "--unset" {
		a.sessionEnv.Unset(sid, rest[1])
		return nil
	}
	a.sessionEnv.Set(sid, rest[0], rest[1])
	return nil
}

func (a *Shux) cliListKeys(out io.Writer) error {
	leader := a.Config.MapLeader
	for _, b := range a.Config.Keymaps.List("prefix") {
		action := string(b.Builtin)
		if action == "" {
			action = "lua"
		}
		if _, err := fmt.Fprintf(out, "%s%s\t%s\t%s\n", leader, b.Key, action, b.Desc); err != nil {
			return err
		}
	}
	return nil
}

func (a *Shux) cliBindKey(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("shux: bind-key requires KEY ACTION")
	}
	key := args[0]
	action := cfg.BuiltinKeyAction(args[1])
	if action == "" {
		return fmt.Errorf("shux: unknown action %q", args[1])
	}
	a.Config.Keymaps.Set("prefix", key, cfg.KeymapBinding{Builtin: action})
	a.notifyClientsKeymaps()
	return nil
}

func (a *Shux) notifyClientsKeymaps() {
	a.clientsMu.Lock()
	programs := make([]*tea.Program, 0, len(a.clients))
	for _, p := range a.clients {
		programs = append(programs, p)
	}
	keymaps := a.Config.Keymaps.Clone()
	a.clientsMu.Unlock()
	msg := ui.KeymapsUpdatedMsg{Keymaps: keymaps}
	for _, p := range programs {
		p.Send(msg)
	}
}

func (a *Shux) cliListBuffers(out io.Writer) error {
	for _, name := range a.buffers.List() {
		label := name
		if label == "" {
			label = "(default)"
		}
		if _, err := fmt.Fprintln(out, label); err != nil {
			return err
		}
	}
	return nil
}

func (a *Shux) cliSetBuffer(args []string) error {
	name := defaultBufferName
	data := []byte(strings.Join(args, " "))
	for i, arg := range args {
		if arg == "-b" && i+1 < len(args) {
			name = args[i+1]
			data = []byte(strings.Join(args[i+2:], " "))
			break
		}
	}
	a.buffers.Set(name, data)
	return nil
}

func (a *Shux) cliPasteBuffer(ctx context.Context, args []string) error {
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	name := defaultBufferName
	for i, arg := range rest {
		if arg == "-b" && i+1 < len(rest) {
			name = rest[i+1]
		}
	}
	data, ok := a.buffers.Get(name)
	if !ok {
		return fmt.Errorf("shux: buffer %q not found", name)
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	return a.supervisor.Send(ctx, protocol.CommandPanePaste{
		SessionID: target.SessionID,
		WindowID:  target.WindowID,
		PaneID:    target.PaneID,
		Data:      data,
	})
}

func (a *Shux) cliResizePane(ctx context.Context, args []string) error {
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	delta := 5
	edge := protocol.PaneResizeEdgeRight
	for _, arg := range rest {
		switch arg {
		case "-L", "left":
			edge = protocol.PaneResizeEdgeLeft
		case "-R", "right":
			edge = protocol.PaneResizeEdgeRight
		case "-U", "up":
			edge = protocol.PaneResizeEdgeUp
		case "-D", "down":
			edge = protocol.PaneResizeEdgeDown
		default:
			if n, err := strconv.Atoi(arg); err == nil {
				delta = n
			}
		}
	}
	return a.supervisor.Send(ctx, protocol.CommandPaneResizeDelta{
		Meta:         a.cliMeta(),
		SessionID:    target.SessionID,
		WindowID:     target.WindowID,
		TargetPaneID: target.PaneID,
		Edge:         edge,
		Delta:        delta,
	})
}

func (a *Shux) cliSwapPane(ctx context.Context, args []string) error {
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	dir := protocol.PaneDirectionLeft
	for _, arg := range rest {
		switch arg {
		case "-D", "down":
			dir = protocol.PaneDirectionDown
		case "-U", "up":
			dir = protocol.PaneDirectionUp
		case "-R", "right":
			dir = protocol.PaneDirectionRight
		case "-L", "left":
			dir = protocol.PaneDirectionLeft
		}
	}
	return a.supervisor.Send(ctx, protocol.CommandPaneSwap{
		Meta:      a.cliMeta(),
		SessionID: target.SessionID,
		WindowID:  target.WindowID,
		PaneID:    target.PaneID,
		Direction: dir,
	})
}

func (a *Shux) cliMovePane(ctx context.Context, args []string) error {
	return a.cliJoinPane(ctx, args)
}

func (a *Shux) cliBreakPane(ctx context.Context, args []string) error {
	targetSpec, _, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	return a.supervisor.Send(ctx, protocol.CommandPaneMove{
		SessionID:      target.SessionID,
		SourceWindowID: target.WindowID,
		TargetWindowID: "",
		PaneID:         target.PaneID,
	})
}

func (a *Shux) cliJoinPane(ctx context.Context, args []string) error {
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	source, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	destSpec := ""
	for i, arg := range rest {
		if arg == "-t" && i+1 < len(rest) {
			destSpec = rest[i+1]
		}
	}
	if destSpec == "" {
		return fmt.Errorf("shux: join-pane requires -t DEST-WINDOW")
	}
	dest, err := a.ResolveCLITarget(ctx, destSpec)
	if err != nil {
		return err
	}
	return a.supervisor.Send(ctx, protocol.CommandPaneMove{
		SessionID:      source.SessionID,
		SourceWindowID: source.WindowID,
		TargetWindowID: dest.WindowID,
		PaneID:         source.PaneID,
	})
}

func (a *Shux) cliSelectLayout(ctx context.Context, args []string) error {
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	preset := protocol.LayoutPresetEvenHorizontal
	for _, arg := range rest {
		switch arg {
		case "even-horizontal":
			preset = protocol.LayoutPresetEvenHorizontal
		case "even-vertical":
			preset = protocol.LayoutPresetEvenVertical
		case "main-horizontal":
			preset = protocol.LayoutPresetMainHorizontal
		}
	}
	return a.supervisor.Send(ctx, protocol.CommandWindowSelectLayout{
		Meta:         a.cliMeta(),
		SessionID:    target.SessionID,
		WindowID:     target.WindowID,
		ActivePaneID: target.PaneID,
		Preset:       preset,
	})
}

func (a *Shux) cliChooseTree(args []string) error {
	mode := ui.ClientUITreeDefault
	for _, arg := range args {
		switch arg {
		case "-s":
			mode = ui.ClientUITreeSessionsCollapsed
		case "-w":
			mode = ui.ClientUITreeWindowsCollapsed
		}
	}
	id, ok := a.clientReg.First()
	if !ok {
		return fmt.Errorf("shux: choose-tree requires an attached client")
	}
	a.sendClientUI(id, ui.ClientUIMsg{Action: ui.ClientUIActionChooseTree, TreeMode: mode})
	return nil
}

func (a *Shux) cliCommandPrompt(args []string) error {
	_ = args
	id, ok := a.clientReg.First()
	if !ok {
		return fmt.Errorf("shux: command-prompt requires an attached client")
	}
	a.sendClientUI(id, ui.ClientUIMsg{Action: ui.ClientUIActionCommandPrompt})
	return nil
}

func (a *Shux) cliDisplayMenu(out io.Writer) error {
	_, err := fmt.Fprintln(out, "shux display-menu: use choose-tree or prefix bindings")
	return err
}

func (a *Shux) sendClientUI(clientID protocol.ClientID, msg ui.ClientUIMsg) {
	a.clientsMu.Lock()
	p := a.clients[clientID]
	a.clientsMu.Unlock()
	if p != nil {
		p.Send(msg)
	}
}
