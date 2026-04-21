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

func main() {
	if err := shux.InitLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to init logger: %v\n", err)
	}

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

	if err := shux.EnsureSessionDir(sessionName); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

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

	shux.Infof("startup: session=%s mode=start-owner", sessionName)
	if err := startOwnerAndWait(sessionName, socketPath, snapshot, opts.Shell, logger); err != nil {
		return fmt.Errorf("failed to start owner: %w", err)
	}

	return runClient(sessionName, socketPath, opts)
}
