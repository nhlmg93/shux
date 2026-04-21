package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"shux/pkg/shux"
)

// OwnerOptions contains options for owner mode.
type OwnerOptions struct {
	SessionName string
	SocketPath  string
}

// startOwnerAndWait spawns the owner process and waits for socket readiness.
func startOwnerAndWait(sessionName, socketPath string, snapshot *shux.SessionSnapshot, shell string, logger shux.ShuxLogger) error {
	_ = shux.CleanupSocket(sessionName)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command(exe, "--owner", sessionName, "--socket", socketPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if snapshot != nil {
		cmd.Env = append(cmd.Env, "SHUX_RESTORE_SNAPSHOT=1")
	}

	cmd.Env = append(cmd.Env, fmt.Sprintf("SHUX_SHELL=%s", shell))

	shux.Infof("startup: spawning owner session=%s pid=%d", sessionName, os.Getpid())
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to spawn owner: %w", err)
	}

	shux.Infof("startup: waiting for owner socket=%s", socketPath)
	if err := shux.WaitForSocket(socketPath, 5*time.Second); err != nil {
		return fmt.Errorf("owner failed to create socket: %w", err)
	}

	shux.Infof("startup: owner ready session=%s", sessionName)
	return nil
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

	if sessionRef == nil {
		shell := os.Getenv("SHUX_SHELL")
		if shell == "" {
			shell = shux.DefaultShell
		}
		sessionRef = shux.StartNamedSessionWithShell(1, sessionName, shell, nil, logger)
		shux.Infof("owner: session=%s created fresh shell=%s", sessionName, shell)
	}

	sessionRef.SetOwnerMode()

	_ = shux.EnsureSessionDir(sessionName)

	snapshotPath := shux.SessionSnapshotPath(sessionName)
	if snapshot == nil {
		snapshot = &shux.SessionSnapshot{
			Version:     shux.SnapshotVersion,
			SessionName: sessionName,
			ID:          1,
			Shell:       sessionRef.GetShell(),
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

	ipcServer, err := shux.NewIPCServer(socketPath, logger)
	if err != nil {
		sessionRef.Shutdown()
		return fmt.Errorf("failed to create IPC server: %w", err)
	}

	ipcServer.Start(func(msg any, reply func(any)) {
		handleOwnerIPC(sessionRef, msg, reply, ipcServer, logger)
	})

	updates := make(chan any, 32)
	sessionRef.Send(shux.SubscribeUpdates{Subscriber: updates})

	shux.Infof("owner: session=%s ready socket=%s", sessionName, socketPath)

	for msg := range updates {
		switch m := msg.(type) {
		case shux.SessionEmpty:
			shux.Infof("owner: session=%s empty, shutting down", sessionName)

			shux.MarkSnapshotDead(snapshot)
			_ = shux.SaveSnapshot(snapshotPath, snapshot, logger)

			ipcServer.BroadcastUpdate(shux.IPCSessionEmpty(m))
			ipcServer.Stop()
			sessionRef.Shutdown()
			return nil
		case shux.PaneContentUpdated:
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

		if err := handleOwnerDetach(sessionRef, logger); err != nil {
			reply(shux.IPCActionResult{Error: err.Error()})
			return
		}
		reply(shux.IPCSessionDetached{})

	case shux.IPCKillSession:

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
	sessionRef.Stop()
}
