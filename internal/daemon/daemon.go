package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"

	"charm.land/wish/v2"
	"github.com/charmbracelet/ssh"
	"shux/internal/client"
	"shux/internal/shux"
	"shux/internal/sshkey"
)

func Run(ctx context.Context, addr string) error {
	if !isLocalAddr(addr) {
		return fmt.Errorf("daemon: refusing non-local bind address %q", addr)
	}
	available, err := client.ServerAvailable(ctx, addr)
	if err != nil {
		return err
	}
	if available {
		return nil
	}

	app, err := shux.NewShux()
	if err != nil {
		return err
	}
	defer app.Close()

	if err := app.Start(ctx); err != nil {
		return err
	}
	if err := app.BootstrapDefaultSession(ctx); err != nil {
		return err
	}

	hostKeyPath, err := sshkey.HostKeyPath()
	if err != nil {
		return err
	}

	srv, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithVersion("shux"),
		wish.WithHostKeyPath(hostKeyPath),
		wish.WithMiddleware(shux.ShuxUiMiddleware(app, &shux.ClientIDSource{})),
	)
	if err != nil {
		return err
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		if err := srv.Shutdown(context.Background()); err != nil {
			return err
		}
		return ctx.Err()
	case <-app.Done():
		if err := srv.Shutdown(context.Background()); err != nil {
			return err
		}
		return nil
	case err := <-errc:
		if errors.Is(err, ssh.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func isLocalAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
