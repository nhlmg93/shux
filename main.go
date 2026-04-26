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

type processRole uint8

const (
	roleClient processRole = iota
	roleDaemonCandidate
	roleInvalid
)

var rootCmd = &cobra.Command{
	Use:     "shux",
	Short:   "shux / \"you shouldn't have\" /",
	Version: "0.1.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		switch detectProcessRole() {
		case roleClient:
			return client.AttachOrSpawn(ctx, defaultAddr)
		case roleDaemonCandidate:
			return daemon.Run(ctx, defaultAddr)
		case roleInvalid:
			return fmt.Errorf("shux requires both stdin and stdout to be interactive terminals")
		default:
			panic("shux: unknown process role")
		}
	},
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}

func detectProcessRole() processRole {
	stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
	stdoutTTY := term.IsTerminal(int(os.Stdout.Fd()))

	switch {
	case stdinTTY && stdoutTTY:
		return roleClient
	case !stdinTTY && !stdoutTTY:
		return roleDaemonCandidate
	default:
		return roleInvalid
	}
}
