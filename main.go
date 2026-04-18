package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/term"
)

func main() {
	// Put terminal in raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	pane1, _ := NewPane(exec.Command("/bin/sh"))
	pane2, _ := NewPane(exec.Command("/bin/sh"))
	defer pane1.Close()
	defer pane2.Close()

	win := NewWindow()
	win.AddPane(pane1)
	win.AddPane(pane2)

	switchOutput := make(chan *Pane)
	go copyOutput(switchOutput)
	switchOutput <- win.Active

	fmt.Println("Ctrl+A 1/2=switch c=create x=kill q=quit")
	runInputLoop(win, switchOutput)
}

func copyOutput(switchPane <-chan *Pane) {
	var cancel func()
	for {
		newPane := <-switchPane
		if cancel != nil {
			cancel()
		}
		ctx, cncl := context.WithCancel(context.Background())
		cancel = cncl
		go func(p *Pane) {
			buf := make([]byte, 1024)
			for {
				select {
				case <-ctx.Done():
					return
				default:
					n, err := p.PTY.TTY.Read(buf)
					if err != nil {
						return
					}
					os.Stdout.Write(buf[:n])
				}
			}
		}(newPane)
	}
}

func runInputLoop(win *Window, switchOutput chan<- *Pane) {
	prefixMode := false
	reader := bufio.NewReader(os.Stdin)

	for {
		select {
		case <-win.Active.Exited:
			fmt.Printf("\n[Pane %d exited]\n", win.Active.ID)
			win.Active.Close()
			if len(win.Panes) == 1 {
				return
			}
			// Remove exited pane and switch to another
			for i, p := range win.Panes {
				if p == win.Active {
					win.Panes = append(win.Panes[:i], win.Panes[i+1:]...)
					break
				}
			}
			win.Active = win.Panes[0]
			switchOutput <- win.Active
		default:
			ch, _ := reader.ReadByte()
			if prefixMode {
				pane, quit := handlePrefixCommand(ch, win)
				if quit {
					return
				}
				if pane != nil {
					switchOutput <- pane
				}
				prefixMode = false
			} else if ch == 1 {
				prefixMode = true
			} else {
				win.Active.PTY.TTY.Write([]byte{ch})
			}
		}
	}
}

func handlePrefixCommand(ch byte, win *Window) (*Pane, bool) {
	switch ch {
	case '1':
		if len(win.Panes) >= 1 {
			win.SetActivePane(win.Panes[0])
			fmt.Printf("\n[Switched to pane %d]\n", win.Active.ID)
			return win.Active, false
		}
	case '2':
		if len(win.Panes) >= 2 {
			win.SetActivePane(win.Panes[1])
			fmt.Printf("\n[Switched to pane %d]\n", win.Active.ID)
			return win.Active, false
		}
	case 'c':
		pane, _ := NewPane(exec.Command("/bin/sh"))
		win.AddPane(pane)
		fmt.Printf("\n[Created pane %d]\n", pane.ID)
		return win.Active, false
	case 'x':
		win.Active.Close()
		fmt.Printf("\n[Killed pane %d]\n", win.Active.ID)
		// Remove killed pane from window
		for i, p := range win.Panes {
			if p == win.Active {
				win.Panes = append(win.Panes[:i], win.Panes[i+1:]...)
				break
			}
		}
		if len(win.Panes) > 0 {
			win.Active = win.Panes[0]
			return win.Active, false
		}
		return nil, true
	case 'q':
		return nil, true
	default:
		win.Active.PTY.TTY.Write([]byte{1, ch})
	}
	return nil, false
}
