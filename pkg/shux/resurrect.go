package shux

import (
	"fmt"

	"github.com/nhlmg93/gotor/actor"
)

// RestoreSessionFromSnapshot loads a session from disk and recreates the actor hierarchy.
func RestoreSessionFromSnapshot(name string, supervisor *actor.Ref) (*actor.Ref, error) {
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

	sessionRef := SpawnSessionFromSnapshot(snapshot, supervisor)

	Infof("restore: session=%s id=%d spawned ref=%p", snapshot.SessionName, snapshot.ID, sessionRef)

	return sessionRef, nil
}
