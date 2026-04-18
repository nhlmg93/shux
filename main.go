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

	prefixMode := false
	reader := bufio.NewReader(os.Stdin)

	for {
		ch, _ := reader.ReadByte()
		if prefixMode {
			switch ch {
			case '1':
				win.SetActivePane(pane1)
				fmt.Printf("\n[Switched to pane %d]\n", pane1.ID)
			case '2':
				win.SetActivePane(pane2)
				fmt.Printf("\n[Switched to pane %d]\n", pane2.ID)
			case 'q':
				return
			default:
				win.Active.PTY.TTY.Write([]byte{1})
				win.Active.PTY.TTY.Write([]byte{ch})
			}
			prefixMode = false
		} else if ch == 1 {
			prefixMode = true
		} else {
			win.Active.PTY.TTY.Write([]byte{ch})
		}
	}
}
