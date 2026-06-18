package shux

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"shux/internal/protocol"
)

const cliCommandTimeout = 2 * time.Second

func (a *Shux) cliMeta() protocol.CommandMeta {
	a.bootstrapReq++
	return protocol.CommandMeta{ClientID: bootstrapClientID, RequestID: a.bootstrapReq}
}

// HandleRemoteCommand executes a detached SSH/CLI command.
func (a *Shux) HandleRemoteCommand(ctx context.Context, command []string, out io.Writer) (bool, error) {
	if len(command) == 0 {
		return false, nil
	}
	switch command[0] {
	case "detach", "detach-client":
		n := a.DetachAllClients()
		_, err := fmt.Fprintf(out, "detached %d client(s)\n", n)
		return true, err
	case "list-sessions":
		return true, a.cliListSessions(ctx, out)
	case "has-session", "has":
		return true, a.cliHasSession(ctx, command[1:])
	case "new-session":
		return true, a.cliNewSession(ctx, command[1:], out)
	case "kill-session":
		return true, a.cliKillSession(ctx, command[1:])
	case "new-window", "neww":
		return true, a.cliNewWindow(ctx, command[1:], out)
	case "kill-window", "killw":
		return true, a.cliKillWindow(ctx, command[1:])
	case "kill-pane", "killp":
		return true, a.cliKillPane(ctx, command[1:])
	case "select-window", "selectw":
		return true, a.cliSelectWindow(ctx, command[1:])
	case "select-pane", "selectp":
		return true, a.cliSelectPane(ctx, command[1:])
	case "split-window", "splitw", "split-pane", "splitp":
		return true, a.cliSplitWindow(ctx, command[1:], out)
	case "send-keys", "send":
		return true, a.cliSendKeys(ctx, command[1:])
	case "capture-pane", "capturep":
		return true, a.cliCapturePane(ctx, command[1:], out)
	case "rename-window", "renamew":
		return true, a.cliRenameWindow(ctx, command[1:], out)
	case "rename-pane":
		return true, a.cliRenamePane(ctx, command[1:], out)
	case "list-windows", "lsw":
		return true, a.cliListWindows(ctx, command[1:], out)
	case "list-panes", "lsp":
		return true, a.cliListPanes(ctx, command[1:], out)
	case "display-message", "display":
		return true, a.cliDisplayMessage(ctx, command[1:], out)
	case "list-commands", "lscm":
		return true, a.cliListCommands(out)
	default:
		return a.handleExtendedCLI(ctx, command[0], command[1:], out)
	}
}

func (a *Shux) cliListSessions(ctx context.Context, out io.Writer) error {
	sessions, err := a.ListSessions(ctx)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		prefix := " "
		if session.SessionID == a.DefaultSessionID {
			prefix = "*"
		}
		if _, err := fmt.Fprintf(out, "%s %s\n", prefix, session.Name); err != nil {
			return err
		}
	}
	return nil
}

func (a *Shux) cliHasSession(ctx context.Context, args []string) error {
	name, err := parseSessionTargetFlag(args)
	if err != nil {
		return err
	}
	_, err = a.ResolveSession(ctx, name)
	return err
}

func (a *Shux) cliNewSession(ctx context.Context, args []string, out io.Writer) error {
	name, err := parseSessionName(args)
	if err != nil {
		return err
	}
	created, err := a.CreateNamedSession(ctx, name)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "%s\n", created.Name)
	return err
}

func (a *Shux) cliKillSession(ctx context.Context, args []string) error {
	name, err := parseKillSessionTarget(args)
	if err != nil {
		return err
	}
	return a.KillSession(ctx, name)
}

func (a *Shux) cliNewWindow(ctx context.Context, args []string, out io.Writer) error {
	targetSpec, _, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	cols, rows := a.windowSize(target.SessionID, target.WindowID)
	if err := a.supervisor.Send(ctx, protocol.CommandCreateWindow{
		SessionID: target.SessionID,
		Cols:      cols,
		Rows:      rows,
		AutoPane:  true,
	}); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "created window")
	return err
}

func (a *Shux) cliKillWindow(ctx context.Context, args []string) error {
	targetSpec, _, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	reply := make(chan struct{}, 1)
	if err := a.supervisor.Send(ctx, protocol.CommandKillWindow{
		SessionID: target.SessionID,
		WindowID:  target.WindowID,
		Reply:     reply,
	}); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-reply:
		return nil
	case <-time.After(cliCommandTimeout):
		return fmt.Errorf("shux: kill-window timed out")
	}
}

func (a *Shux) cliKillPane(ctx context.Context, args []string) error {
	targetSpec, _, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	return a.supervisor.Send(ctx, protocol.CommandPaneClose{
		SessionID: target.SessionID,
		WindowID:  target.WindowID,
		PaneID:    target.PaneID,
	})
}

func (a *Shux) cliSelectWindow(ctx context.Context, args []string) error {
	targetSpec, _, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	a.applyDefaultTarget(target)
	return nil
}

func (a *Shux) cliSelectPane(ctx context.Context, args []string) error {
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target := a.defaultCLITarget()
	if targetSpec != "" {
		target, err = a.ResolveCLITarget(ctx, targetSpec)
		if err != nil {
			return err
		}
	}
	for _, arg := range rest {
		switch arg {
		case "-L", "-l", "left":
			return a.cliFocusDirection(ctx, target, protocol.PaneFocusLeft)
		case "-R", "-r", "right":
			return a.cliFocusDirection(ctx, target, protocol.PaneFocusRight)
		case "-U", "-u", "up":
			return a.cliFocusDirection(ctx, target, protocol.PaneFocusUp)
		case "-D", "-d", "down":
			return a.cliFocusDirection(ctx, target, protocol.PaneFocusDown)
		default:
			if protocol.PaneID(arg).Valid() {
				t, err := a.targetFromPaneID(ctx, arg)
				if err != nil {
					return err
				}
				target = t
			}
		}
	}
	a.applyDefaultTarget(target)
	return nil
}

func (a *Shux) cliFocusDirection(ctx context.Context, target CLITarget, dir protocol.PaneFocusDirection) error {
	return a.supervisor.Send(ctx, protocol.CommandPaneFocus{
		Meta:          a.cliMeta(),
		SessionID:     target.SessionID,
		WindowID:      target.WindowID,
		CurrentPaneID: target.PaneID,
		Direction:     dir,
	})
}

func (a *Shux) cliSplitWindow(ctx context.Context, args []string, out io.Writer) error {
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	dir := protocol.SplitVertical
	for _, arg := range rest {
		switch arg {
		case "-h", "-H", "horizontal":
			dir = protocol.SplitVertical
		case "-v", "-V", "vertical":
			dir = protocol.SplitHorizontal
		}
	}
	if err := a.supervisor.Send(ctx, protocol.CommandPaneSplit{
		Meta:         a.cliMeta(),
		SessionID:    target.SessionID,
		WindowID:     target.WindowID,
		TargetPaneID: target.PaneID,
		Direction:    dir,
	}); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "split pane")
	return err
}

func (a *Shux) cliSendKeys(ctx context.Context, args []string) error {
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return fmt.Errorf("shux: send-keys requires at least one key")
	}
	for _, token := range rest {
		if err := a.cliSendOneKey(ctx, target, token); err != nil {
			return err
		}
	}
	return nil
}

func (a *Shux) cliSendOneKey(ctx context.Context, target CLITarget, token string) error {
	keyName, text, isKey := parseSendKeyToken(token)
	if isKey {
		return a.supervisor.Send(ctx, protocol.CommandPaneKey{
			SessionID: target.SessionID,
			WindowID:  target.WindowID,
			PaneID:    target.PaneID,
			Action:    protocol.KeyActionPress,
			Key:       keyName,
			Text:      text,
		})
	}
	return a.supervisor.Send(ctx, protocol.CommandPanePaste{
		SessionID: target.SessionID,
		WindowID:  target.WindowID,
		PaneID:    target.PaneID,
		Data:      []byte(token),
	})
}

func parseSendKeyToken(token string) (keyName, text string, isKey bool) {
	switch strings.ToLower(token) {
	case "enter", "return":
		return "enter", "\r", true
	case "escape", "esc":
		return "escape", "", true
	case "space":
		return "space", " ", true
	case "tab":
		return "tab", "\t", true
	case "bspace", "backspace":
		return "backspace", "", true
	case "dc", "delete":
		return "delete", "", true
	case "up", "down", "left", "right", "home", "end", "pageup", "pagedown":
		return strings.ToLower(token), "", true
	}
	if strings.HasPrefix(token, "C-") && len(token) == 3 {
		return "ctrl+" + strings.ToLower(string(token[2])), "", true
	}
	return "", "", false
}

func (a *Shux) cliCapturePane(ctx context.Context, args []string, out io.Writer) error {
	targetSpec, _, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	_ = ctx
	for _, screen := range a.cache.ScreenSnapshots(target.SessionID, target.WindowID) {
		if screen.PaneID != target.PaneID {
			continue
		}
		_, err = fmt.Fprint(out, screenText(screen, controlCaptureMaxBytes))
		return err
	}
	return fmt.Errorf("shux: no screen snapshot for pane %q", target.PaneID)
}

func (a *Shux) cliRenameWindow(ctx context.Context, args []string, out io.Writer) error {
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(strings.Join(rest, " "))
	if name == "" {
		return fmt.Errorf("shux: rename-window requires a name")
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	if err := a.supervisor.Send(ctx, protocol.CommandWindowRename{
		SessionID: target.SessionID,
		WindowID:  target.WindowID,
		Name:      name,
	}); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "renamed window")
	return err
}

func (a *Shux) cliRenamePane(ctx context.Context, args []string, out io.Writer) error {
	targetSpec, rest, err := ParseTargetFlag(args)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(strings.Join(rest, " "))
	if name == "" {
		return fmt.Errorf("shux: rename-pane requires a name")
	}
	target, err := a.ResolveCLITarget(ctx, targetSpec)
	if err != nil {
		return err
	}
	if err := a.supervisor.Send(ctx, protocol.CommandPaneRename{
		SessionID: target.SessionID,
		WindowID:  target.WindowID,
		PaneID:    target.PaneID,
		Name:      name,
	}); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "renamed pane")
	return err
}

func (a *Shux) cliListWindows(ctx context.Context, args []string, out io.Writer) error {
	jsonOut, rest, err := parseJSONFlag(args)
	if err != nil {
		return err
	}
	targetSpec, _, err := ParseTargetFlag(rest)
	if err != nil {
		return err
	}
	sid, err := a.sessionIDFromTargetSpec(ctx, targetSpec)
	if err != nil {
		return err
	}
	windows := a.ListWindowsForSession(sid)
	if jsonOut {
		return json.NewEncoder(out).Encode(windows)
	}
	if _, err := fmt.Fprintln(out, "INDEX\tSESSION\tWINDOW\tPANES"); err != nil {
		return err
	}
	for _, w := range windows {
		if _, err := fmt.Fprintf(out, "%d\t%s\t%s\t%d\n", w.Index, w.SessionID, w.WindowID, w.PaneCount); err != nil {
			return err
		}
	}
	return nil
}

func (a *Shux) cliListPanes(ctx context.Context, args []string, out io.Writer) error {
	jsonOut, rest, err := parseJSONFlag(args)
	if err != nil {
		return err
	}
	targetSpec, _, err := ParseTargetFlag(rest)
	if err != nil {
		return err
	}
	sid, err := a.sessionIDFromTargetSpec(ctx, targetSpec)
	if err != nil {
		return err
	}
	panes := a.ListPanesForSession(sid)
	if jsonOut {
		return json.NewEncoder(out).Encode(panes)
	}
	if _, err := fmt.Fprintln(out, "INDEX\tSESSION\tWINDOW\tWIN_INDEX\tPANE\tCOL\tROW\tCOLS\tROWS"); err != nil {
		return err
	}
	for _, p := range panes {
		if _, err := fmt.Fprintf(out, "%d\t%s\t%s\t%d\t%s\t%d\t%d\t%d\t%d\n",
			p.Index, p.SessionID, p.WindowID, p.WindowIndex, p.PaneID, p.Col, p.Row, p.Cols, p.Rows); err != nil {
			return err
		}
	}
	return nil
}

func (a *Shux) sessionIDFromTargetSpec(ctx context.Context, spec string) (protocol.SessionID, error) {
	if spec == "" {
		return a.DefaultSessionID, nil
	}
	if strings.HasPrefix(spec, "s-") && protocol.SessionID(spec).Valid() {
		return protocol.SessionID(spec), nil
	}
	sess, err := a.ResolveSession(ctx, spec)
	if err != nil {
		return "", err
	}
	return sess.SessionID, nil
}

func (a *Shux) cliDisplayMessage(ctx context.Context, args []string, out io.Writer) error {
	jsonOut, rest, err := parseJSONFlag(args)
	if err != nil {
		return err
	}
	targetSpec, rest, err := ParseTargetFlag(rest)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return fmt.Errorf("shux: display-message requires FORMAT")
	}
	format := strings.Join(rest, " ")
	msgCtx := a.DisplayMessageContext()
	if targetSpec != "" {
		target, err := a.ResolveCLITarget(ctx, targetSpec)
		if err != nil {
			return err
		}
		msgCtx = a.DisplayMessageContextFor(target.SessionID, target.WindowID, target.PaneID)
	}
	msg := FormatDisplayMessage(format, msgCtx)
	if jsonOut {
		return json.NewEncoder(out).Encode(protocol.DisplayMessageInfo{
			Message:               msg,
			DisplayMessageContext: msgCtx,
		})
	}
	_, err = fmt.Fprintln(out, msg)
	return err
}

func (a *Shux) cliListCommands(out io.Writer) error {
	for _, cmd := range []string{
		"attach", "attach-session", "detach", "detach-client",
		"new-session", "kill-session", "has-session", "list-sessions",
		"new-window", "kill-window", "select-window", "list-windows",
		"split-window", "kill-pane", "select-pane", "list-panes",
		"send-keys", "capture-pane", "display-message",
		"rename-window", "rename-pane", "list-commands",
		"kill-server", "source-file", "list-clients", "switch-client",
		"show-options", "set-option", "show-environment", "set-environment",
		"list-keys", "bind-key", "list-buffers", "set-buffer", "paste-buffer",
		"resize-pane", "swap-pane", "move-pane", "break-pane", "join-pane",
		"select-layout", "choose-tree", "command-prompt", "display-menu",
		"query", "control-mode", "restart", "restart-daemon",
	} {
		if _, err := fmt.Fprintln(out, cmd); err != nil {
			return err
		}
	}
	return nil
}

func parseSessionTargetFlag(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "-t" || args[i] == "--target" || args[i] == "-s" || args[i] == "--session" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("shux: missing session target for %s", args[i])
			}
			name := strings.TrimSpace(args[i+1])
			if !protocol.ValidSessionName(name) {
				return "", fmt.Errorf("shux: invalid session target %q", name)
			}
			return name, nil
		}
	}
	return "", fmt.Errorf("shux: requires -t SESSION")
}
