package shux

import "fmt"

// RestoreSessionFromSnapshot loads a session from disk and recreates the loop hierarchy.
func RestoreSessionFromSnapshot(name string, notify func(any)) (*SessionRef, error) {
	path := SessionSnapshotPath(name)
	Infof("restore: begin session=%s path=%s", name, path)
	snapshot, err := LoadSnapshot(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot: %w", err)
	}
	if snapshot.SessionName == "" {
		snapshot.SessionName = name
	}

	Infof("restore: session=%s id=%d shell=%s windows=%d", snapshot.SessionName, snapshot.ID, snapshot.Shell, len(snapshot.Windows))

	sessionRef := StartSessionFromSnapshot(snapshot, notify)

	Infof("restore: session=%s id=%d started ref=%p", snapshot.SessionName, snapshot.ID, sessionRef)

	return sessionRef, nil
}
