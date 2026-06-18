package shux

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"shux/internal/protocol"
)

// CLITarget resolves a tmux-style -t target to shux IDs.
type CLITarget struct {
	SessionID protocol.SessionID
	WindowID  protocol.WindowID
	PaneID    protocol.PaneID
}

func (a *Shux) defaultCLITarget() CLITarget {
	return CLITarget{
		SessionID: a.DefaultSessionID,
		WindowID:  a.DefaultWindowID,
		PaneID:    a.DefaultPaneID,
	}
}

func (a *Shux) applyDefaultTarget(t CLITarget) {
	if t.SessionID.Valid() {
		a.DefaultSessionID = t.SessionID
	}
	if t.WindowID.Valid() {
		a.DefaultWindowID = t.WindowID
	}
	if t.PaneID.Valid() {
		a.DefaultPaneID = t.PaneID
	}
}

// ParseTargetFlag extracts -t/--target from argv.
func ParseTargetFlag(args []string) (target string, rest []string, err error) {
	return parseFlagValue(args, "-t", "--target")
}

// ParseClientFlag extracts -c/--client from argv.
func ParseClientFlag(args []string) (client string, rest []string, err error) {
	return parseFlagValue(args, "-c", "--client")
}

// ParseSourceTargetFlags extracts -s/--source and -t/--target from argv.
func ParseSourceTargetFlags(args []string) (source, target string, rest []string, err error) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-s", "--source":
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("shux: missing value for %s", args[i])
			}
			source = strings.TrimSpace(args[i+1])
			i++
		case "-t", "--target":
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("shux: missing value for %s", args[i])
			}
			target = strings.TrimSpace(args[i+1])
			i++
		default:
			rest = append(rest, args[i])
		}
	}
	return source, target, rest, nil
}

func parseFlagValue(args []string, names ...string) (value string, rest []string, err error) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case names[0], names[1]:
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("shux: missing value for %s", args[i])
			}
			value = strings.TrimSpace(args[i+1])
			i++
		default:
			rest = append(rest, args[i])
		}
	}
	return value, rest, nil
}

func (a *Shux) ResolveCLITarget(ctx context.Context, spec string) (CLITarget, error) {
	if spec == "" {
		return a.defaultCLITarget(), nil
	}
	spec = strings.TrimSpace(spec)
	if strings.HasPrefix(spec, "s-") && protocol.SessionID(spec).Valid() {
		return a.targetFromSessionID(ctx, protocol.SessionID(spec))
	}
	if strings.HasPrefix(spec, "w-") && protocol.WindowID(spec).Valid() {
		return a.targetFromWindowID(ctx, spec)
	}
	if strings.HasPrefix(spec, "p-") && protocol.PaneID(spec).Valid() {
		return a.targetFromPaneID(ctx, spec)
	}

	sessionName := spec
	windowIndex := 0
	paneIndex := 0
	if i := strings.Index(spec, ":"); i >= 0 {
		sessionName = spec[:i]
		rest := spec[i+1:]
		if j := strings.Index(rest, "."); j >= 0 {
			w, err := strconv.Atoi(rest[:j])
			if err != nil || w < 1 {
				return CLITarget{}, fmt.Errorf("shux: invalid window index in target %q", spec)
			}
			windowIndex = w
			p, err := strconv.Atoi(rest[j+1:])
			if err != nil || p < 1 {
				return CLITarget{}, fmt.Errorf("shux: invalid pane index in target %q", spec)
			}
			paneIndex = p
		} else if rest != "" {
			w, err := strconv.Atoi(strings.TrimPrefix(rest, "#"))
			if err != nil || w < 1 {
				return CLITarget{}, fmt.Errorf("shux: invalid window index in target %q", spec)
			}
			windowIndex = w
		}
	}

	sess, err := a.ResolveSession(ctx, sessionName)
	if err != nil {
		return CLITarget{}, err
	}
	out := CLITarget{SessionID: sess.SessionID}
	windowIDs := a.cache.WindowIDs(sess.SessionID)
	if len(windowIDs) == 0 {
		return CLITarget{}, fmt.Errorf("shux: session %q has no windows", sessionName)
	}
	if windowIndex == 0 {
		out.WindowID = windowIDs[0]
	} else if windowIndex > len(windowIDs) {
		return CLITarget{}, fmt.Errorf("shux: window index %d out of range for session %q", windowIndex, sessionName)
	} else {
		out.WindowID = windowIDs[windowIndex-1]
	}
	layout, ok := a.cache.LayoutSnapshot(sess.SessionID, out.WindowID)
	if !ok || len(layout.Panes) == 0 {
		return CLITarget{}, fmt.Errorf("shux: window %q has no panes", out.WindowID)
	}
	if paneIndex == 0 {
		out.PaneID = layout.Panes[0].PaneID
	} else if paneIndex > len(layout.Panes) {
		return CLITarget{}, fmt.Errorf("shux: pane index %d out of range", paneIndex)
	} else {
		out.PaneID = layout.Panes[paneIndex-1].PaneID
	}
	return out, nil
}

func (a *Shux) targetFromSessionID(ctx context.Context, sid protocol.SessionID) (CLITarget, error) {
	name, ok := a.cache.SessionName(sid)
	if !ok {
		return CLITarget{}, fmt.Errorf("shux: unknown session id %q", sid)
	}
	return a.ResolveCLITarget(ctx, name)
}

func (a *Shux) targetFromWindowID(ctx context.Context, windowID string) (CLITarget, error) {
	wid := protocol.WindowID(windowID)
	sessions, err := a.ListSessions(ctx)
	if err != nil {
		return CLITarget{}, err
	}
	for _, sess := range sessions {
		for _, id := range a.cache.WindowIDs(sess.SessionID) {
			if id != wid {
				continue
			}
			layout, ok := a.cache.LayoutSnapshot(sess.SessionID, wid)
			if !ok || len(layout.Panes) == 0 {
				return CLITarget{}, fmt.Errorf("shux: window %q has no panes", wid)
			}
			return CLITarget{SessionID: sess.SessionID, WindowID: wid, PaneID: layout.Panes[0].PaneID}, nil
		}
	}
	return CLITarget{}, fmt.Errorf("shux: unknown window %q", wid)
}

func (a *Shux) targetFromPaneID(ctx context.Context, paneID string) (CLITarget, error) {
	pid := protocol.PaneID(paneID)
	sessions, err := a.ListSessions(ctx)
	if err != nil {
		return CLITarget{}, err
	}
	for _, sess := range sessions {
		for _, wid := range a.cache.WindowIDs(sess.SessionID) {
			layout, ok := a.cache.LayoutSnapshot(sess.SessionID, wid)
			if !ok {
				continue
			}
			for _, p := range layout.Panes {
				if p.PaneID == pid {
					return CLITarget{SessionID: sess.SessionID, WindowID: wid, PaneID: pid}, nil
				}
			}
		}
	}
	return CLITarget{}, fmt.Errorf("shux: unknown pane %q", pid)
}

func (a *Shux) windowSize(sessionID protocol.SessionID, windowID protocol.WindowID) (uint16, uint16) {
	layout, ok := a.cache.LayoutSnapshot(sessionID, windowID)
	if !ok || layout.Cols == 0 || layout.Rows == 0 {
		return 80, 24
	}
	return uint16(layout.Cols), uint16(layout.Rows)
}
