package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"shux/internal/client"
	"shux/internal/daemon"
)

const defaultAddr = "127.0.0.1:23234"

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
		return client.AttachOrSpawn(cmd.Context(), defaultAddr)
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
	rootCmd.AddCommand(attachCmd, detachCmd)
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}

func runRoot(ctx context.Context) error {
	if isInteractiveTerminal() {
		return client.AttachOrSpawn(ctx, defaultAddr)
	}

	if isDaemonChild() {
		return daemon.Run(ctx, defaultAddr)
	}

	return fmt.Errorf("shux requires an interactive terminal")
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) &&
		term.IsTerminal(int(os.Stdout.Fd()))
}

func isDaemonChild() bool {
	return !term.IsTerminal(int(os.Stdin.Fd())) &&
		!term.IsTerminal(int(os.Stdout.Fd()))
}
