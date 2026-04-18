package main

import (
	"io"
	"os"
	"os/exec"
)

func main() {
	pane, err := NewPane(exec.Command("/bin/sh"))
	if err != nil {
		panic(err)
	}
	defer pane.Close()

	go io.Copy(pane.PTY.TTY, os.Stdin)
	io.Copy(os.Stdout, pane.PTY.TTY)
}
