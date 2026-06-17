package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"shux/internal/client"
	"shux/internal/daemon"
	"shux/internal/lua"
)

var bashShell bool

var rootCmd = &cobra.Command{
	Use:     "shux",
	Short:   "shux / \"you shouldn't have\" /",
	Version: "0.1.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRoot(cmd.Context())
	},
}

var attachCmd = &cobra.Command{
	Use:     "attach",
	Aliases: []string{"a", "attach-session"},
	Short:   "Attach to the shux session",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAttach(cmd.Context())
	},
}

var detachCmd = &cobra.Command{
	Use:     "detach",
	Aliases: []string{"detach-client"},
	Short:   "Detach shux clients",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDetach(cmd.Context())
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&bashShell, "bash", false, "use /bin/bash for panes when spawning a new daemon; ignored when attaching to an existing daemon")
	rootCmd.AddCommand(attachCmd, detachCmd)
}

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

func runRoot(ctx context.Context) error {
	if isInteractiveTerminal() {
		return runAttach(ctx)
	}
	if isDaemonChild() {
		return daemon.Run(ctx, loadOpts())
	}
	return fmt.Errorf("shux requires an interactive terminal")
}

func runAttach(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.AttachOrSpawnWithOptions(ctx, addr, attachOptions())
}

func runDetach(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.Detach(ctx, addr)
}

func attachOptions() client.AttachOptions {
	return client.AttachOptions{Bash: bashShell}
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) &&
		term.IsTerminal(int(os.Stdout.Fd()))
}

func isDaemonChild() bool {
	return !term.IsTerminal(int(os.Stdin.Fd())) &&
		!term.IsTerminal(int(os.Stdout.Fd()))
}
