package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"golang.org/x/sys/unix"
	"shux/pkg/shux"
)

// OwnerOptions contains options for owner mode.
type OwnerOptions struct {
	SessionName string
	SocketPath  string
}

func main() {
	if err := shux.InitLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to init logger: %v\n", err)
	}

	// Check if running in hidden owner mode
	if len(os.Args) >= 3 && os.Args[1] == "--owner" {
		opts := OwnerOptions{
			SessionName: os.Args[2],
		}
		if len(os.Args) >= 5 && os.Args[3] == "--socket" {
			opts.SocketPath = os.Args[4]
		}
		if err := runOwner(opts); err != nil {
			shux.Errorf("owner: fatal error: %v", err)
			os.Exit(1)
		}
		return
	}

	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runApp(opts RunOptions) error {
	sessionName := opts.SessionName
	snapshotPath := shux.SessionSnapshotPath(sessionName)
	socketPath := shux.SessionSocketPath(sessionName)
	logger := shux.DefaultShuxLogger()

	shux.Infof("startup: session=%s shell=%s snapshot=%s socket=%s", sessionName, opts.Shell, snapshotPath, socketPath)

	// Ensure session directory exists
	if err := shux.EnsureSessionDir(sessionName); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Phase 1: Try to attach to live owner
	var snapshot *shux.SessionSnapshot
	if shux.SessionSnapshotExists(sessionName) {
		var err error
		snapshot, err = shux.LoadSnapshot(snapshotPath, logger)
		if err != nil {
			shux.Warnf("startup: session=%s failed to load snapshot: %v", sessionName, err)
		} else if shux.IsLiveOwner(snapshot) {
			shux.Infof("startup: session=%s found live owner pid=%d", sessionName, snapshot.OwnerPID)
			if _, err := shux.TryDialLiveOwner(snapshot); err == nil {
				shux.Infof("startup: session=%s mode=attach-live", sessionName)
				return runClient(sessionName, socketPath, opts)
			}
			shux.Warnf("startup: session=%s live owner unreachable, will restore", sessionName)
		}
	}

	// Phase 2: Start new owner (from snapshot or fresh)
	shux.Infof("startup: session=%s mode=start-owner", sessionName)
	if err := startOwnerAndWait(sessionName, socketPath, snapshot, opts.Shell, logger); err != nil {
		return fmt.Errorf("failed to start owner: %w", err)
	}

	// Phase 3: Attach as client
	return runClient(sessionName, socketPath, opts)
}

// startOwnerAndWait spawns the owner process and waits for socket readiness.
func startOwnerAndWait(sessionName, socketPath string, snapshot *shux.SessionSnapshot, shell string, logger shux.ShuxLogger) error {
	// Clean up any stale socket
	_ = shux.CleanupSocket(sessionName)

	// Build owner command
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command(exe, "--owner", sessionName, "--socket", socketPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	// If we have a snapshot, pass it via environment (owner will load it)
	if snapshot != nil {
		cmd.Env = append(cmd.Env, "SHUX_RESTORE_SNAPSHOT=1")
	}

	// Pass shell preference
	cmd.Env = append(cmd.Env, fmt.Sprintf("SHUX_SHELL=%s", shell))

	shux.Infof("startup: spawning owner session=%s pid=%d", sessionName, os.Getpid())
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to spawn owner: %w", err)
	}

	// Wait for socket to become available
	shux.Infof("startup: waiting for owner socket=%s", socketPath)
	if err := shux.WaitForSocket(socketPath, 5*time.Second); err != nil {
		return fmt.Errorf("owner failed to create socket: %w", err)
	}

	shux.Infof("startup: owner ready session=%s", sessionName)
	return nil
}

// runClient connects to a live owner and runs the UI.
func runClient(sessionName, socketPath string, opts RunOptions) error {
	logger := shux.DefaultShuxLogger()

	// Create remote session reference
	remoteRef, err := shux.NewRemoteSessionRef(socketPath, sessionName, logger)
	if err != nil {
		return fmt.Errorf("failed to connect to session owner: %w", err)
	}
	defer remoteRef.Shutdown()

	shux.Infof("client: session=%s connected", sessionName)
	return runSessionRemote(sessionName, remoteRef, opts.Keymap, opts.MouseEnabled, opts.StartupWarnings)
}

// runOwner runs the owner process (hidden mode).
func runOwner(opts OwnerOptions) error {
	logger := shux.DefaultShuxLogger()
	sessionName := opts.SessionName
	socketPath := opts.SocketPath

	shux.Infof("owner: session=%s starting pid=%d", sessionName, os.Getpid())

	// Determine if restoring from snapshot
	var sessionRef *shux.SessionRef
	var snapshot *shux.SessionSnapshot

	if os.Getenv("SHUX_RESTORE_SNAPSHOT") == "1" && shux.SessionSnapshotExists(sessionName) {
		var err error
		snapshot, err = shux.LoadSnapshot(shux.SessionSnapshotPath(sessionName), logger)
		if err != nil {
			shux.Warnf("owner: session=%s failed to load snapshot: %v", sessionName, err)
		} else {
			sessionRef, err = shux.RestoreSessionFromSnapshot(sessionName, nil, logger)
			if err != nil {
				shux.Errorf("owner: session=%s failed to restore: %v", sessionName, err)
				return err
			}
			shux.Infof("owner: session=%s restored windows=%d", sessionName, len(snapshot.Windows))
		}
	}

	// Create fresh session if not restored
	if sessionRef == nil {
		shell := os.Getenv("SHUX_SHELL")
		if shell == "" {
			shell = shux.DefaultShell
		}
		sessionRef = shux.StartNamedSessionWithShell(1, sessionName, shell, nil, logger)
		shux.Infof("owner: session=%s created fresh shell=%s", sessionName, shell)
	}

	// Set owner mode so detach saves but doesn't kill the session
	sessionRef.SetOwnerMode()

	// Ensure session directory exists
	_ = shux.EnsureSessionDir(sessionName)

	// Mark snapshot as live
	snapshotPath := shux.SessionSnapshotPath(sessionName)
	if snapshot == nil {
		snapshot = &shux.SessionSnapshot{
			Version:     shux.SnapshotVersion,
			SessionName: sessionName,
			ID:          1,
			Shell:       sessionRef.GetShell(), // We'll need to add this method
		}
	}

	if err := shux.MarkSnapshotLive(snapshot, os.Getpid(), socketPath); err != nil {
		shux.Warnf("owner: session=%s failed to mark live: %v", sessionName, err)
	} else {
		if err := shux.SaveSnapshot(snapshotPath, snapshot, logger); err != nil {
			shux.Warnf("owner: session=%s failed to save live snapshot: %v", sessionName, err)
		} else {
			shux.Infof("owner: session=%s marked live pid=%d", sessionName, os.Getpid())
		}
	}

	// Create IPC server
	ipcServer, err := shux.NewIPCServer(socketPath, logger)
	if err != nil {
		sessionRef.Shutdown()
		return fmt.Errorf("failed to create IPC server: %w", err)
	}

	// Set up IPC handler
	ipcServer.Start(func(msg any, reply func(any)) {
		handleOwnerIPC(sessionRef, msg, reply, ipcServer, logger)
	})

	// Subscribe to session updates to forward to clients
	updates := make(chan any, 32)
	sessionRef.Send(shux.SubscribeUpdates{Subscriber: updates})

	shux.Infof("owner: session=%s ready socket=%s", sessionName, socketPath)

	// Main owner loop: handle session updates and forward to clients
	for msg := range updates {
		switch m := msg.(type) {
		case shux.SessionEmpty:
			shux.Infof("owner: session=%s empty, shutting down", sessionName)
			// Clear live status
			shux.MarkSnapshotDead(snapshot)
			_ = shux.SaveSnapshot(snapshotPath, snapshot, logger)
			// Notify clients and cleanup
			ipcServer.BroadcastUpdate(shux.IPCSessionEmpty(m))
			ipcServer.Stop()
			sessionRef.Shutdown()
			return nil
		case shux.PaneContentUpdated:
			// Forward content update to all clients
			ipcServer.BroadcastUpdate(shux.IPCPaneContentUpdated(m))
		}
	}
	return nil
}

// handleOwnerIPC handles IPC messages from clients.
func handleOwnerIPC(sessionRef *shux.SessionRef, msg any, reply func(any), ipcServer *shux.IPCServer, logger shux.ShuxLogger) {
	switch m := msg.(type) {
	case shux.IPCActionMsg:
		result := sessionRef.Ask(shux.ActionMsg(m))
		if r, ok := <-result; ok && r != nil {
			if ar, ok := r.(shux.ActionResult); ok {
				var errStr string
				if ar.Err != nil {
					errStr = ar.Err.Error()
				}
				reply(shux.IPCActionResult{Quit: ar.Quit, Error: errStr})
				return
			}
		}
		reply(shux.IPCActionResult{})

	case shux.IPCKeyInput:
		sessionRef.Send(shux.KeyInput(m))
		return

	case shux.IPCMouseInput:
		sessionRef.Send(shux.MouseInput(m))
		return

	case shux.IPCWriteToPane:
		sessionRef.Send(shux.WriteToPane(m))
		return

	case shux.IPCResizeMsg:
		sessionRef.Send(shux.ResizeMsg(m))
		return

	case shux.IPCCreateWindow:
		sessionRef.Send(shux.CreateWindow(m))
		return

	case shux.IPCGetWindowView:
		result := sessionRef.Ask(shux.GetWindowView{})
		if r, ok := <-result; ok && r != nil {
			if view, ok := r.(shux.WindowView); ok {
				reply(shux.IPCWindowView(view))
				return
			}
		}
		reply(shux.IPCWindowView{})

	case shux.IPCGetActiveWindow:
		result := sessionRef.Ask(shux.GetActiveWindow{})
		if r, ok := <-result; ok && r != nil {
			reply(true)
			return
		}
		reply(false)

	case shux.GetFullSessionSnapshot:
		// Owner-only: return complete snapshot with all windows
		result := sessionRef.Ask(shux.GetFullSessionSnapshot{})
		if r, ok := <-result; ok && r != nil {
			if snapshot, ok := r.(*shux.SessionSnapshot); ok {
				reply(*snapshot)
				return
			}
		}
		reply(shux.SessionSnapshot{})

	case shux.IPCExecuteCommandMsg:
		result := sessionRef.Ask(shux.ExecuteCommandMsg(m))
		if r, ok := <-result; ok && r != nil {
			if cr, ok := r.(shux.CommandResult); ok {
				var errStr string
				if cr.Error != "" {
					errStr = cr.Error
				}
				reply(shux.IPCCommandResult{
					Success: cr.Success,
					Error:   errStr,
					Quit:    cr.Quit,
				})
				return
			}
		}
		reply(shux.IPCCommandResult{})

	case shux.IPCDetachSession:
		// Handle detach: save snapshot, keep owner running
		if err := handleOwnerDetach(sessionRef, logger); err != nil {
			reply(shux.IPCActionResult{Error: err.Error()})
			return
		}
		reply(shux.IPCSessionDetached{})

	case shux.IPCKillSession:
		// Handle kill: stop everything
		handleOwnerKill(sessionRef, ipcServer, logger)
		reply(shux.IPCSessionKilled{})

	case shux.IPCSubscribeUpdates:
		return

	case shux.IPCUnsubscribeUpdates:
		return

	default:
		if logger != nil {
			logger.Warnf("owner: unknown IPC message type %T", msg)
		}
		reply(nil)
	}
}

// handleOwnerDetach saves snapshot but keeps owner running.
func handleOwnerDetach(sessionRef *shux.SessionRef, logger shux.ShuxLogger) error {
	// Get full snapshot including all windows
	result := sessionRef.Ask(shux.GetFullSessionSnapshot{})
	r, ok := <-result
	if !ok || r == nil {
		return fmt.Errorf("failed to get session snapshot")
	}
	snapshot, ok := r.(*shux.SessionSnapshot)
	if !ok {
		return fmt.Errorf("invalid snapshot type")
	}

	path := shux.SessionSnapshotPath(snapshot.SessionName)
	if err := shux.SaveSnapshot(path, snapshot, logger); err != nil {
		return err
	}

	if logger != nil {
		logger.Infof("owner: detach saved snapshot session=%s windows=%d", snapshot.SessionName, len(snapshot.Windows))
	}
	return nil
}

// handleOwnerKill stops the session completely.
func handleOwnerKill(sessionRef *shux.SessionRef, ipcServer *shux.IPCServer, logger shux.ShuxLogger) {
	// Stop all windows
	sessionRef.Stop()
	// Server will detect session empty and clean up
}

func detectTerminalSize() (width, height int, ok bool) {
	fds := []uintptr{os.Stdout.Fd(), os.Stdin.Fd()}
	for _, fd := range fds {
		ws, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
		if err != nil || ws == nil || ws.Col == 0 || ws.Row == 0 {
			continue
		}
		return int(ws.Col), int(ws.Row), true
	}
	return 0, 0, false
}

// runSessionRemote runs the UI with a remote session reference.
func runSessionRemote(sessionName string, remoteRef *shux.RemoteSessionRef, keymap shux.Keymap, mouseEnabled bool, startupWarnings []string) error {
	shux.Infof("ui: session=%s starting remote program", sessionName)
	model := shux.NewModelWithStartupWarnings(remoteRef, keymap, mouseEnabled, startupWarnings)
	opts := []tea.ProgramOption{}
	if os.Getenv("COLORTERM") == "truecolor" || os.Getenv("COLORTERM") == "24bit" {
		opts = append(opts, tea.WithColorProfile(colorprofile.TrueColor))
	}
	if width, height, ok := detectTerminalSize(); ok {
		shux.Infof("ui: bootstrap terminal size %dx%d", width, height)
		model.SetInitialSize(width, height)
		opts = append(opts, tea.WithWindowSize(width, height))
		if existing := <-remoteRef.Ask(shux.GetActiveWindow{}); existing == nil {
			shux.Infof("ui: bootstrap creating initial window %dx%d", height, width)
			remoteRef.Send(shux.CreateWindow{Rows: height, Cols: width})
		} else {
			shux.Infof("ui: bootstrap resizing existing session to %dx%d", height, width)
			remoteRef.Send(shux.ResizeMsg{Rows: height, Cols: width})
		}
	}
	p := tea.NewProgram(model, opts...)

	updates := make(chan any, 32)
	remoteRef.Send(shux.SubscribeUpdates{Subscriber: updates})
	defer remoteRef.Send(shux.UnsubscribeUpdates{Subscriber: updates})

	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case msg := <-updates:
				switch msg.(type) {
				case shux.PaneContentUpdated, shux.SessionEmpty:
					p.Send(shux.UpdateMsg{})
				}
			}
		}
	}()
	go func() {
		delays := []time.Duration{0, 50 * time.Millisecond, 150 * time.Millisecond, 400 * time.Millisecond}
		for _, delay := range delays {
			if delay > 0 {
				time.Sleep(delay)
			}
			p.Send(shux.UpdateMsg{})
		}
	}()

	_, err := p.Run()
	if err != nil {
		shux.Warnf("ui: session=%s program exited with err=%v", sessionName, err)
	} else {
		shux.Infof("ui: session=%s program exited cleanly", sessionName)
	}
	return err
}
