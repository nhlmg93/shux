package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"shux/internal/lua"
)

func main() {
	cobra.CheckErr(rootCmd.Execute())
}

func loadOpts() lua.LoadOptions {
	return lua.LoadOptions{Bash: bashShell}
}

func bindAddr() (string, error) {
	rt, err := lua.Load(loadOpts())
	if err != nil {
		return "", err
	}
	addr := rt.Config.WithDefaults().BindAddr
	rt.Close()
	return addr, nil
}

func configStateDir() (string, error) {
	rt, err := lua.Load(loadOpts())
	if err != nil {
		return "", err
	}
	dir := rt.Config.WithDefaults().StateDir
	rt.Close()
	if dir == "" {
		return "", fmt.Errorf("shux: empty state_dir in config")
	}
	return dir, nil
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) &&
		term.IsTerminal(int(os.Stdout.Fd()))
}

func isDaemonChild() bool {
	return !term.IsTerminal(int(os.Stdin.Fd())) &&
		!term.IsTerminal(int(os.Stdout.Fd()))
}
