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
				default:
					handled, err := app.HandleRemoteCommand(sess.Context(), command, sess)
					if err != nil {
						wish.Fatalln(sess, err)
						return
					}
					if handled {
						return
					}
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
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			jsonOut = true
		case "-t", "--target", "-s", "--session":
			if i+1 >= len(args) {
				return false, nil, fmt.Errorf("shux: missing value for %s", arg)
			}
			rest = append(rest, arg, args[i+1])
			i++
		default:
			if strings.HasPrefix(arg, "-") {
				return false, nil, fmt.Errorf("shux: unknown flag %q", arg)
			}
			rest = append(rest, arg)
		}
	}
	return jsonOut, rest, nil
}

func sessionIDForQuery(app *Shux, ctx context.Context, req protocol.QueryRequest) (protocol.SessionID, error) {
	if req.Target != "" {
		target, err := app.ResolveCLITarget(ctx, req.Target)
		if err != nil {
			return "", err
		}
		return target.SessionID, nil
	}
	if req.SessionName != "" {
		sess, err := app.ResolveSession(ctx, req.SessionName)
		if err != nil {
			return "", err
		}
		return sess.SessionID, nil
	}
	return app.DefaultSessionID, nil
}

func runQueryRPC(app *Shux, sess ssh.Session) (protocol.QueryResponse, error) {
	var req protocol.QueryRequest
	if err := json.NewDecoder(sess).Decode(&req); err != nil {
		return protocol.QueryResponse{}, fmt.Errorf("shux: decode query request: %w", err)
	}
	ctx := sess.Context()
	switch req.Method {
	case protocol.QueryListWindows:
		sid, err := sessionIDForQuery(app, ctx, req)
		if err != nil {
			return protocol.QueryResponse{}, err
		}
		return protocol.QueryResponse{Windows: app.ListWindowsForSession(sid)}, nil
	case protocol.QueryListPanes:
		sid, err := sessionIDForQuery(app, ctx, req)
		if err != nil {
			return protocol.QueryResponse{}, err
		}
		return protocol.QueryResponse{Panes: app.ListPanesForSession(sid)}, nil
	case protocol.QueryDisplayMessage:
		if req.Format == "" {
			return protocol.QueryResponse{}, fmt.Errorf("shux: display-message requires FORMAT")
		}
		msgCtx := app.DisplayMessageContext()
		if req.Target != "" {
			target, err := app.ResolveCLITarget(ctx, req.Target)
			if err != nil {
				return protocol.QueryResponse{}, err
			}
			msgCtx = app.DisplayMessageContextFor(target.SessionID, target.WindowID, target.PaneID)
		} else if req.SessionName != "" {
			sid, err := sessionIDForQuery(app, ctx, req)
			if err != nil {
				return protocol.QueryResponse{}, err
			}
			msgCtx = app.DisplayMessageContextFor(sid, "", "")
		}
		return protocol.QueryResponse{
			Display: &protocol.DisplayMessageInfo{
				Message:               FormatDisplayMessage(req.Format, msgCtx),
				DisplayMessageContext: msgCtx,
			},
		}, nil
	case protocol.QueryHasSession:
		name := req.SessionName
		if name == "" && req.Target != "" {
			name = req.Target
		}
		if name == "" {
			return protocol.QueryResponse{}, fmt.Errorf("shux: has-session requires session name")
		}
		_, err := app.ResolveSession(ctx, name)
		exists := err == nil
		return protocol.QueryResponse{Exists: &exists}, nil
	case protocol.QueryCapturePane:
		target, err := app.ResolveCLITarget(ctx, req.Target)
		if err != nil {
			return protocol.QueryResponse{}, err
		}
		screens := app.cache.ScreenSnapshots(target.SessionID, target.WindowID)
		for _, screen := range screens {
			if screen.PaneID != target.PaneID {
				continue
			}
			text := screenText(screen, controlCaptureMaxBytes)
			return protocol.QueryResponse{
				Capture: &protocol.CapturePaneInfo{
					SessionID: target.SessionID,
					WindowID:  target.WindowID,
					PaneID:    target.PaneID,
					Text:      text,
				},
			}, nil
		}
		return protocol.QueryResponse{}, fmt.Errorf("shux: no screen snapshot for pane %q", target.PaneID)
	case protocol.QueryCheckpointState:
		pruned := app.checkpoint()
		return protocol.QueryResponse{Checkpoint: &protocol.StateCheckpointInfo{Pruned: pruned}}, nil
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

func parseKillSessionTarget(args []string) (string, error) {
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
	return "", fmt.Errorf("shux: kill-session requires -t NAME")
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

