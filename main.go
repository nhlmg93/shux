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
var listWindowsJSON bool
var listPanesJSON bool
var displayMessageJSON bool
var controlMode bool
var attachTarget string
var sessionName string

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

var newSessionCmd = &cobra.Command{
	Use:   "new-session",
	Short: "Create a new named session",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNewSession(cmd.Context())
	},
}

var listSessionsCmd = &cobra.Command{
	Use:   "list-sessions",
	Short: "List daemon session names",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runListSessions(cmd.Context())
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

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Gracefully restart the shux daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRestart(cmd.Context())
	},
}

var listWindowsCmd = &cobra.Command{
	Use:   "list-windows",
	Short: "List windows from the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runListWindows(cmd.Context(), listWindowsJSON)
	},
}

var listPanesCmd = &cobra.Command{
	Use:   "list-panes",
	Short: "List panes from the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runListPanes(cmd.Context(), listPanesJSON)
	},
}

var displayMessageCmd = &cobra.Command{
	Use:   "display-message FORMAT",
	Short: "Render a message from daemon introspection format variables",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDisplayMessage(cmd.Context(), args[0], displayMessageJSON)
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&bashShell, "bash", false, "use /bin/bash for panes when spawning a new daemon; ignored when attaching to an existing daemon")
	listWindowsCmd.Flags().BoolVar(&listWindowsJSON, "json", false, "print machine-readable JSON")
	listPanesCmd.Flags().BoolVar(&listPanesJSON, "json", false, "print machine-readable JSON")
	displayMessageCmd.Flags().BoolVar(&displayMessageJSON, "json", false, "print machine-readable JSON")
	attachCmd.Flags().BoolVarP(&controlMode, "control", "C", false, "attach in experimental line-oriented control mode")
	attachCmd.Flags().StringVarP(&attachTarget, "target", "t", "", "attach to named session")
	newSessionCmd.Flags().StringVarP(&sessionName, "session", "s", "", "session name")
	_ = newSessionCmd.MarkFlagRequired("session")
	rootCmd.AddCommand(attachCmd, detachCmd, restartCmd, newSessionCmd, listSessionsCmd, listWindowsCmd, listPanesCmd, displayMessageCmd)
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

func runRestart(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.Restart(ctx, addr)
}

func runNewSession(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.NewSession(ctx, addr, attachOptions(), sessionName)
}

func runListSessions(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	sessions, err := client.ListSessions(ctx, addr, attachOptions())
	if err != nil {
		return err
	}
	for _, session := range sessions {
		fmt.Println(session)
	}
	return nil
}

func runListWindows(ctx context.Context, jsonOutput bool) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.ListWindows(ctx, addr, jsonOutput)
}

func runListPanes(ctx context.Context, jsonOutput bool) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.ListPanes(ctx, addr, jsonOutput)
}

func runDisplayMessage(ctx context.Context, format string, jsonOutput bool) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.DisplayMessage(ctx, addr, format, jsonOutput)
}

func attachOptions() client.AttachOptions {
	return client.AttachOptions{Bash: bashShell, Control: controlMode, TargetSession: attachTarget}
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) &&
		term.IsTerminal(int(os.Stdout.Fd()))
}

func isDaemonChild() bool {
	return !term.IsTerminal(int(os.Stdin.Fd())) &&
		!term.IsTerminal(int(os.Stdout.Fd()))
}
