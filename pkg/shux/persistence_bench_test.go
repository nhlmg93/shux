package shux

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type persistenceBenchShape struct {
	name           string
	windows        int
	panesPerWindow int
}

var persistenceBenchShapes = []persistenceBenchShape{
	{name: "small", windows: 1, panesPerWindow: 1},
	{name: "medium", windows: 4, panesPerWindow: 4},
}

func BenchmarkSaveSnapshot(b *testing.B) {
	b.ReportAllocs()

	for _, shape := range persistenceBenchShapes {
		shape := shape
		b.Run(shape.name, func(b *testing.B) {
			dir := b.TempDir()
			snapshot := benchmarkSnapshot(shape, "/bin/sh", dir)
			path := filepath.Join(dir, "save.gob")

			if err := SaveSnapshot(path, snapshot); err != nil {
				b.Fatalf("initial SaveSnapshot: %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				b.Fatalf("stat snapshot: %v", err)
			}
			b.SetBytes(info.Size())

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := SaveSnapshot(path, snapshot); err != nil {
					b.Fatalf("SaveSnapshot: %v", err)
				}
			}
		})
	}
}

func BenchmarkLoadSnapshot(b *testing.B) {
	b.ReportAllocs()

	for _, shape := range persistenceBenchShapes {
		shape := shape
		b.Run(shape.name, func(b *testing.B) {
			dir := b.TempDir()
			snapshot := benchmarkSnapshot(shape, "/bin/sh", dir)
			path := filepath.Join(dir, "load.gob")

			if err := SaveSnapshot(path, snapshot); err != nil {
				b.Fatalf("prepare snapshot: %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				b.Fatalf("stat snapshot: %v", err)
			}
			b.SetBytes(info.Size())

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				loaded, err := LoadSnapshot(path)
				if err != nil {
					b.Fatalf("LoadSnapshot: %v", err)
				}
				if loaded == nil {
					b.Fatal("LoadSnapshot returned nil snapshot")
				}
			}
		})
	}
}

func BenchmarkDetachSession(b *testing.B) {
	b.ReportAllocs()

	for _, shape := range persistenceBenchShapes {
		shape := shape
		b.Run(shape.name, func(b *testing.B) {
			dir := b.TempDir()
			template := benchmarkSnapshot(shape, "/bin/sh", dir)
			path := filepath.Join(dir, "detach.gob")

			if err := SaveSnapshot(path, template); err != nil {
				b.Fatalf("prepare snapshot: %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				b.Fatalf("stat snapshot: %v", err)
			}
			b.SetBytes(info.Size())

			b.StopTimer()
			for i := 0; i < b.N; i++ {
				session := startBenchmarkLiveSession(b, template)

				b.StartTimer()
				snapshot := session.buildSnapshot()
				if err := SaveSnapshot(path, snapshot); err != nil {
					b.Fatalf("SaveSnapshot: %v", err)
				}
				b.StopTimer()

				shutdownBenchmarkSession(session)
			}
		})
	}
}

func BenchmarkAttachSession(b *testing.B) {
	b.ReportAllocs()

	for _, shape := range persistenceBenchShapes {
		shape := shape
		b.Run(shape.name, func(b *testing.B) {
			dir := b.TempDir()
			template := benchmarkSnapshot(shape, "/bin/sh", dir)
			path := filepath.Join(dir, "attach.gob")

			if err := SaveSnapshot(path, template); err != nil {
				b.Fatalf("prepare snapshot: %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				b.Fatalf("stat snapshot: %v", err)
			}
			b.SetBytes(info.Size())

			b.StopTimer()
			for i := 0; i < b.N; i++ {
				b.StartTimer()
				loaded, err := LoadSnapshot(path)
				if err != nil {
					b.Fatalf("LoadSnapshot: %v", err)
				}
				session := restoreBenchmarkSession(b, loaded)
				b.StopTimer()

				shutdownBenchmarkSession(session)
			}
		})
	}
}

func benchmarkSnapshot(shape persistenceBenchShape, shell, cwd string) *SessionSnapshot {
	windows := make([]WindowSnapshot, 0, shape.windows)
	windowOrder := make([]uint32, 0, shape.windows)

	for windowID := 1; windowID <= shape.windows; windowID++ {
		paneOrder := make([]uint32, 0, shape.panesPerWindow)
		panes := make([]PaneSnapshot, 0, shape.panesPerWindow)

		for paneID := 1; paneID <= shape.panesPerWindow; paneID++ {
			id := uint32(paneID)
			paneOrder = append(paneOrder, id)
			panes = append(panes, PaneSnapshot{
				ID:          id,
				Shell:       shell,
				Rows:        24,
				Cols:        80,
				CWD:         cwd,
				WindowTitle: fmt.Sprintf("win-%d-pane-%d", windowID, paneID),
			})
		}

		wid := uint32(windowID)
		windowOrder = append(windowOrder, wid)
		windows = append(windows, WindowSnapshot{
			ID:         wid,
			ActivePane: uint32(shape.panesPerWindow),
			PaneOrder:  paneOrder,
			Panes:      panes,
		})
	}

	activeWindow := uint32(1)
	if len(windowOrder) > 0 {
		activeWindow = windowOrder[len(windowOrder)-1]
	}

	return &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  fmt.Sprintf("bench-%s", shape.name),
		ID:           1,
		Shell:        shell,
		ActiveWindow: activeWindow,
		WindowOrder:  windowOrder,
		Windows:      windows,
	}
}

func startBenchmarkLiveSession(b *testing.B, snapshot *SessionSnapshot) *Session {
	b.Helper()

	session := &Session{
		id:          snapshot.ID,
		name:        snapshot.SessionName,
		windows:     make(map[uint32]*WindowRef),
		windowOrder: make([]uint32, 0, len(snapshot.WindowOrder)),
		active:      snapshot.ActiveWindow,
		shell:       snapshot.Shell,
	}

	windowsByID := make(map[uint32]WindowSnapshot, len(snapshot.Windows))
	for _, winSnap := range snapshot.Windows {
		windowsByID[winSnap.ID] = winSnap
		if winSnap.ID > session.windowID {
			session.windowID = winSnap.ID
		}
	}

	for _, winID := range snapshot.WindowOrder {
		winSnap, ok := windowsByID[winID]
		if !ok {
			continue
		}

		winRef := StartWindow(winSnap.ID, nil)
		session.windows[winSnap.ID] = winRef
		session.windowOrder = append(session.windowOrder, winSnap.ID)

		panesByID := make(map[uint32]PaneSnapshot, len(winSnap.Panes))
		for _, paneSnap := range winSnap.Panes {
			panesByID[paneSnap.ID] = paneSnap
		}
		for _, paneID := range winSnap.PaneOrder {
			paneSnap, ok := panesByID[paneID]
			if !ok {
				continue
			}
			winRef.Send(CreatePane{
				ID:    paneSnap.ID,
				Rows:  paneSnap.Rows,
				Cols:  paneSnap.Cols,
				Shell: paneSnap.Shell,
				CWD:   paneSnap.CWD,
			})
		}
		if activeIdx := indexOfOrderedID(winSnap.PaneOrder, winSnap.ActivePane); activeIdx > 0 {
			winRef.Send(SwitchToPane{Index: activeIdx})
		}
	}

	waitForBenchmarkSessionReady(b, session, snapshot)
	return session
}

func restoreBenchmarkSession(b *testing.B, snapshot *SessionSnapshot) *Session {
	b.Helper()

	session := &Session{
		id:          snapshot.ID,
		name:        normalizeSessionName(snapshot.SessionName),
		windows:     make(map[uint32]*WindowRef),
		windowOrder: nil,
		active:      0,
		shell:       normalizeShell(snapshot.Shell),
		snapshot:    snapshot,
	}

	session.restoreFromSnapshot()
	waitForBenchmarkSessionReady(b, session, snapshot)
	return session
}

func waitForBenchmarkSessionReady(b *testing.B, session *Session, expected *SessionSnapshot) {
	b.Helper()

	if !pollFor(5*time.Second, func() bool {
		if len(session.windowOrder) != len(expected.WindowOrder) {
			return false
		}
		if session.active != expected.ActiveWindow {
			return false
		}

		for _, winSnap := range expected.Windows {
			winRef := session.windows[winSnap.ID]
			if winRef == nil {
				return false
			}
			result, ok := askValue(winRef, GetWindowSnapshotData{})
			winData, ok := result.(WindowSnapshot)
			if !ok {
				return false
			}
			if winData.ActivePane != winSnap.ActivePane {
				return false
			}
			if len(winData.PaneOrder) != len(winSnap.PaneOrder) || len(winData.Panes) != len(winSnap.Panes) {
				return false
			}
			for i, pane := range winData.Panes {
				expectedPane := winSnap.Panes[i]
				if pane.ID != expectedPane.ID {
					return false
				}
				if pane.CWD != expectedPane.CWD {
					return false
				}
			}
		}
		return true
	}) {
		b.Fatalf("timeout waiting for benchmark session ready: windows=%d panes/window=%d", len(expected.WindowOrder), panesPerWindow(expected))
	}
}

func shutdownBenchmarkSession(session *Session) {
	if session == nil {
		return
	}
	for _, winID := range session.windowOrder {
		if win := session.windows[winID]; win != nil {
			win.Shutdown()
		}
	}
}

func panesPerWindow(snapshot *SessionSnapshot) int {
	if snapshot == nil || len(snapshot.Windows) == 0 {
		return 0
	}
	return len(snapshot.Windows[0].Panes)
}
