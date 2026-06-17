package shux

import (
	"context"
	"fmt"
	"time"

	"shux/internal/client"
)

const restartSpawnTimeout = 5 * time.Second

// BeginGracefulRestart checkpoints state and returns after acknowledging the request.
// The caller should invoke FinishGracefulRestart asynchronously to release the listen
// socket, spawn the replacement daemon, and shut down this instance.
func (a *Shux) BeginGracefulRestart() error {
	if a.getState() != stateReady {
		return fmt.Errorf("shux: restart before ready")
	}
	a.checkpoint()
	return nil
}

// FinishGracefulRestart stops this daemon and hands off to a freshly spawned replacement.
func (a *Shux) FinishGracefulRestart(ctx context.Context, opts client.AttachOptions) error {
	a.DetachAllClients()
	if a.restartShutdown != nil {
		if err := a.restartShutdown(ctx); err != nil {
			return fmt.Errorf("shux: restart shutdown: %w", err)
		}
	}
	addr := a.Config.WithDefaults().BindAddr
	if err := client.SpawnAndWaitReady(ctx, addr, opts, restartSpawnTimeout); err != nil {
		return fmt.Errorf("shux: restart spawn: %w", err)
	}
	a.Logger.Info("shux: graceful restart complete")
	a.RequestShutdown()
	return nil
}
