package shux

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/wish/v2"
	wishtea "charm.land/wish/v2/bubbletea"
	"github.com/charmbracelet/ssh"
	"shux/internal/cfg"
	"shux/internal/client"
	"shux/internal/protocol"
)

type ClientIDSource struct {
	next atomic.Uint64
}

func (s *ClientIDSource) Next() protocol.ClientID {
	if s == nil {
		panic("shux: nil client id source")
	}
	id := s.next.Add(1)
	return protocol.ClientID(fmt.Sprintf("ssh-%d", id))
}

func ShuxUiMiddleware(app *Shux, ids *ClientIDSource) wish.Middleware {
	if app == nil {
		panic("shux: nil app")
	}
	if ids == nil {
		panic("shux: nil client id source")
	}
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			targetSessionID := app.DefaultSessionID
			if command := sess.Command(); len(command) > 0 {
				switch command[0] {
				case "detach", "detach-client":
					n := app.DetachAllClients()
					_, _ = fmt.Fprintf(sess, "detached %d client(s)\n", n)
					return
				case "restart", "restart-daemon":
					if err := app.BeginGracefulRestart(); err != nil {
						wish.Fatalln(sess, err)
						return
					}
					_, _ = fmt.Fprintln(sess, "restarting shux daemon...")
					time.Sleep(50 * time.Millisecond)
					opts := client.AttachOptions{Bash: app.Config.ShellPath == cfg.BashShellPath}
					if err := app.FinishGracefulRestart(context.Background(), opts); err != nil {
						wish.Fatalln(sess, err)
					}
					return
				case "list-windows":
					jsonOut, _, err := parseJSONFlag(command[1:])
					if err != nil {
						wish.Fatalln(sess, err)
						return
					}
					windows := app.ListWindows()
					if jsonOut {
						if err := json.NewEncoder(sess).Encode(windows); err != nil {
							wish.Fatalln(sess, err)
							return
						}
						return
					}
					_, _ = fmt.Fprintln(sess, "INDEX\tSESSION\tWINDOW\tPANES")
					for _, window := range windows {
						_, _ = fmt.Fprintf(sess, "%d\t%s\t%s\t%d\n", window.Index, window.SessionID, window.WindowID, window.PaneCount)
					}
					return
				case "list-panes":
					jsonOut, _, err := parseJSONFlag(command[1:])
					if err != nil {
						wish.Fatalln(sess, err)
						return
					}
					panes := app.ListPanes()
					if jsonOut {
						if err := json.NewEncoder(sess).Encode(panes); err != nil {
							wish.Fatalln(sess, err)
							return
						}
						return
					}
					_, _ = fmt.Fprintln(sess, "INDEX\tSESSION\tWINDOW\tWIN_INDEX\tPANE\tCOL\tROW\tCOLS\tROWS")
					for _, pane := range panes {
						_, _ = fmt.Fprintf(sess, "%d\t%s\t%s\t%d\t%s\t%d\t%d\t%d\t%d\n",
							pane.Index,
							pane.SessionID,
							pane.WindowID,
							pane.WindowIndex,
							pane.PaneID,
							pane.Col,
							pane.Row,
							pane.Cols,
							pane.Rows,
						)
					}
					return
				case "display-message":
					jsonOut, args, err := parseJSONFlag(command[1:])
					if err != nil {
						wish.Fatalln(sess, err)
						return
					}
					if len(args) == 0 {
						wish.Fatalln(sess, "shux: display-message requires FORMAT")
						return
					}
					format := strings.Join(args, " ")
					ctx := app.DisplayMessageContext()
					msg := FormatDisplayMessage(format, ctx)
					if jsonOut {
						payload := map[string]any{
							"message":      msg,
							"session_id":   ctx.SessionID,
							"window_id":    ctx.WindowID,
							"window_index": ctx.WindowIndex,
							"pane_id":      ctx.PaneID,
							"pane_index":   ctx.PaneIndex,
						}
						if err := json.NewEncoder(sess).Encode(payload); err != nil {
							wish.Fatalln(sess, err)
							return
						}
						return
					}
					_, _ = fmt.Fprintln(sess, msg)
					return
				case "query":
					if len(command[1:]) > 0 {
						wish.Fatalln(sess, "shux: query does not accept command arguments")
						return
					}
					resp, err := runQueryRPC(app, sess)
					if err != nil {
						wish.Fatalln(sess, err)
						return
					}
					if err := json.NewEncoder(sess).Encode(resp); err != nil {
						wish.Fatalln(sess, err)
						return
					}
					return
				case "control-mode":
					if err := app.RunControlMode(sess.Context(), ids.Next(), sess, sess); err != nil {
						wish.Fatalln(sess, err)
					}
					return
				case "new-session":
					name, err := parseSessionName(command[1:])
					if err != nil {
						wish.Fatalln(sess, err)
						return
					}
					created, err := app.CreateNamedSession(sess.Context(), name)
					if err != nil {
						wish.Fatalln(sess, err)
						return
					}
					_, _ = fmt.Fprintf(sess, "%s\n", created.Name)
					return
				case "list-sessions":
					sessions, err := app.ListSessions(sess.Context())
					if err != nil {
						wish.Fatalln(sess, err)
						return
					}
					for _, session := range sessions {
						prefix := " "
						if session.SessionID == app.DefaultSessionID {
							prefix = "*"
						}
						_, _ = fmt.Fprintf(sess, "%s %s\n", prefix, session.Name)
					}
					return
				case "attach", "attach-session":
					targetName, err := parseAttachTarget(command[1:])
					if err != nil {
						wish.Fatalln(sess, err)
						return
					}
					if targetName != "" {
						target, err := app.ResolveSession(sess.Context(), targetName)
						if err != nil {
							wish.Fatalln(sess, err)
							return
						}
						targetSessionID = target.SessionID
					}
				case "rename-window":
					if len(command) < 2 {
						wish.Fatalln(sess, "usage: rename-window <name>")
						return
					}
					name := strings.TrimSpace(strings.Join(command[1:], " "))
					if err := app.supervisor.Send(sess.Context(), protocol.CommandWindowRename{
						SessionID: app.DefaultSessionID,
						WindowID:  app.DefaultWindowID,
						Name:      name,
					}); err != nil {
						wish.Fatalln(sess, err)
						return
					}
					_, _ = fmt.Fprintln(sess, "renamed window")
					return
				case "rename-pane":
					if len(command) < 2 {
						wish.Fatalln(sess, "usage: rename-pane <name>")
						return
					}
					name := strings.TrimSpace(strings.Join(command[1:], " "))
					if err := app.supervisor.Send(sess.Context(), protocol.CommandPaneRename{
						SessionID: app.DefaultSessionID,
						WindowID:  app.DefaultWindowID,
						PaneID:    app.DefaultPaneID,
						Name:      name,
					}); err != nil {
						wish.Fatalln(sess, err)
						return
					}
					_, _ = fmt.Fprintln(sess, "renamed pane")
					return
				default:
					wish.Fatalln(sess, "shux: unknown command")
					return
				}
			}

			_, windowChanges, ok := sess.Pty()
			if !ok {
				wish.Fatalln(sess, "shux requires an interactive PTY")
				return
			}

			ctx, cancel := context.WithCancel(sess.Context())
			defer cancel()

			p, cleanup, err := app.NewClientProgramForSession(ctx, ids.Next(), targetSessionID, wishtea.MakeOptions(sess)...)
			if err != nil {
				wish.Fatalln(sess, err)
				return
			}
			defer cleanup()

			go func() {
				for {
					select {
					case <-ctx.Done():
						p.Quit()
						return
					case w, ok := <-windowChanges:
						if !ok {
							return
						}
						p.Send(tea.WindowSizeMsg{Width: w.Width, Height: w.Height})
					}
				}
			}()

			if _, err := p.Run(); err != nil {
				wish.Fatalln(sess, err)
				return
			}
			p.Kill()
			if next != nil {
				next(sess)
			}
		}
	}
}

func parseJSONFlag(args []string) (bool, []string, error) {
	jsonOut := false
	rest := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			if strings.HasPrefix(arg, "-") {
				return false, nil, fmt.Errorf("shux: unknown flag %q", arg)
			}
			rest = append(rest, arg)
		}
	}
	return jsonOut, rest, nil
}

func runQueryRPC(app *Shux, sess ssh.Session) (protocol.QueryResponse, error) {
	var req protocol.QueryRequest
	if err := json.NewDecoder(sess).Decode(&req); err != nil {
		return protocol.QueryResponse{}, fmt.Errorf("shux: decode query request: %w", err)
	}
	switch req.Method {
	case protocol.QueryListWindows:
		return protocol.QueryResponse{Windows: app.ListWindows()}, nil
	case protocol.QueryListPanes:
		return protocol.QueryResponse{Panes: app.ListPanes()}, nil
	case protocol.QueryDisplayMessage:
		if req.Format == "" {
			return protocol.QueryResponse{}, fmt.Errorf("shux: display-message requires FORMAT")
		}
		ctx := app.DisplayMessageContext()
		return protocol.QueryResponse{
			Display: &protocol.DisplayMessageInfo{
				Message:               FormatDisplayMessage(req.Format, ctx),
				DisplayMessageContext: ctx,
			},
		}, nil
	default:
		return protocol.QueryResponse{}, fmt.Errorf("shux: unknown query method %q", req.Method)
	}
}

func parseSessionName(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "-s" || args[i] == "--session" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("shux: missing session name for %s", args[i])
			}
			name := strings.TrimSpace(args[i+1])
			if !protocol.ValidSessionName(name) {
				return "", fmt.Errorf("shux: invalid session name %q", name)
			}
			return name, nil
		}
	}
	return "", fmt.Errorf("shux: new-session requires -s NAME")
}

func parseAttachTarget(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "-t" || args[i] == "--target" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("shux: missing attach target for %s", args[i])
			}
			name := strings.TrimSpace(args[i+1])
			if !protocol.ValidSessionName(name) {
				return "", fmt.Errorf("shux: invalid session target %q", name)
			}
			return name, nil
		}
	}
	return "", nil
}

