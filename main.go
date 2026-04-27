package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"shux/internal/client"
	"shux/internal/daemon"
	"shux/internal/shux"
)

const defaultAddr = "127.0.0.1:23234"

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
		return client.AttachOrSpawnWithOptions(cmd.Context(), defaultAddr, attachOptions())
	},
}

var detachCmd = &cobra.Command{
	Use:     "detach",
	Aliases: []string{"detach-client"},
	Short:   "Detach shux clients",
	RunE: func(cmd *cobra.Command, args []string) error {
		return client.Detach(cmd.Context(), defaultAddr)
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&bashShell, "bash", false, "use /bin/bash for panes when spawning a new daemon; ignored when attaching to an existing daemon")
	rootCmd.AddCommand(attachCmd, detachCmd)
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}

func runRoot(ctx context.Context) error {
	if isInteractiveTerminal() {
		return client.AttachOrSpawnWithOptions(ctx, defaultAddr, attachOptions())
	}

	if isDaemonChild() {
		return daemon.RunWithConfig(ctx, defaultAddr, runtimeConfig())
	}

	return fmt.Errorf("shux requires an interactive terminal")
}

func attachOptions() client.AttachOptions {
	return client.AttachOptions{Bash: bashShell}
}

func runtimeConfig() shux.Config {
	if bashShell {
		return shux.BashConfig()
	}
	return shux.DefaultConfig()
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) &&
		term.IsTerminal(int(os.Stdout.Fd()))
}

func isDaemonChild() bool {
	return !term.IsTerminal(int(os.Stdin.Fd())) &&
		!term.IsTerminal(int(os.Stdout.Fd()))
}
