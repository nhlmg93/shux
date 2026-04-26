package shux

import (
	"context"
	"fmt"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"charm.land/wish/v2"
	wishtea "charm.land/wish/v2/bubbletea"
	"github.com/charmbracelet/ssh"
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
			_, windowChanges, ok := sess.Pty()
			if !ok {
				wish.Fatalln(sess, "shux requires an interactive PTY")
				return
			}

			ctx, cancel := context.WithCancel(sess.Context())
			defer cancel()

			p, cleanup, err := app.NewClientProgram(ctx, ids.Next(), wishtea.MakeOptions(sess)...)
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
