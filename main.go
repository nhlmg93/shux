package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func main() {
	pane1, _ := NewPane(exec.Command("/bin/sh"))
	pane2, _ := NewPane(exec.Command("/bin/sh"))
	defer pane1.Close()
	defer pane2.Close()

	win := NewWindow()
	win.AddPane(pane1)
	win.AddPane(pane2)

	go io.Copy(os.Stdout, win.Active.PTY.TTY)

	fmt.Println("Ctrl+A 1/2 to switch panes, Ctrl+A q to quit")
	runInputLoop(win)
}

func runInputLoop(win *Window) {
	prefixMode := false
	reader := bufio.NewReader(os.Stdin)

	for {
		select {
		case <-win.Active.Exited:
			fmt.Printf("\n[Pane %d exited]\n", win.Active.ID)
			return
		default:
			ch, _ := reader.ReadByte()
			if prefixMode {
				if handlePrefixCommand(ch, win) {
					return
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

func handlePrefixCommand(ch byte, win *Window) bool {
	switch ch {
	case '1':
		win.SetActivePane(win.Panes[0])
		fmt.Printf("\n[Switched to pane %d]\n", win.Active.ID)
	case '2':
		win.SetActivePane(win.Panes[1])
		fmt.Printf("\n[Switched to pane %d]\n", win.Active.ID)
	case 'q':
		return true
	default:
		win.Active.PTY.TTY.Write([]byte{1, ch})
	}
	return false
}
