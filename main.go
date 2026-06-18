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

var killSessionCmd = &cobra.Command{
	Use:   "kill-session",
	Short: "Close a session and remove it from the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runKillSession(cmd.Context())
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

var renameWindowCmd = &cobra.Command{
	Use:   "rename-window <name>",
	Short: "Rename active window",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRenameWindow(cmd.Context(), args[0])
	},
}

var renamePaneCmd = &cobra.Command{
	Use:   "rename-pane <name>",
	Short: "Rename active pane",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRenamePane(cmd.Context(), args[0])
	},
}

var hasSessionCmd = &cobra.Command{
	Use:   "has-session",
	Short: "Test whether a session exists",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHasSession(cmd.Context())
	},
}

var newWindowCmd = &cobra.Command{
	Use:   "new-window",
	Short: "Create a new window",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNewWindow(cmd.Context())
	},
}

var killWindowCmd = &cobra.Command{
	Use:   "kill-window",
	Short: "Close a window",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runKillWindow(cmd.Context())
	},
}

var killPaneCmd = &cobra.Command{
	Use:   "kill-pane",
	Short: "Close a pane",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runKillPane(cmd.Context())
	},
}

var selectWindowCmd = &cobra.Command{
	Use:   "select-window",
	Short: "Select a window",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSelectWindow(cmd.Context())
	},
}

var splitWindowCmd = &cobra.Command{
	Use:   "split-window",
	Short: "Split the active pane",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSplitWindow(cmd.Context())
	},
}

var sendKeysCmd = &cobra.Command{
	Use:   "send-keys [keys...]",
	Short: "Send keys to a pane",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSendKeys(cmd.Context(), args)
	},
}

var capturePaneCmd = &cobra.Command{
	Use:   "capture-pane",
	Short: "Capture pane contents",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCapturePane(cmd.Context())
	},
}

var listCommandsCmd = &cobra.Command{
	Use:   "list-commands",
	Short: "List available remote commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runListCommands(cmd.Context())
	},
}

var killServerCmd = remoteCLICommand("kill-server", "kill-server", "Shut down the shux daemon")

var sourceFileCmd = remoteCLICommand("source-file", "source-file PATH", "Reload shux configuration from a Lua file", cobra.ExactArgs(1))

var listClientsCmd = remoteCLICommand("list-clients", "list-clients", "List attached clients")

var switchClientCmd = remoteCLICommandWithFlags("switch-client", "switch-client", "Switch the attached client to another session", remoteCLIFlags{requireTarget: true, client: true})

var showOptionsCmd = remoteCLICommand("show-options", "show-options [option]", "Show daemon options")

var setOptionCmd = remoteCLICommand("set-option", "set-option OPTION VALUE", "Set a runtime option", cobra.ExactArgs(2))

var showEnvironmentCmd = remoteCLICommand("show-environment", "show-environment [session]", "Show session environment variables")

var setEnvironmentCmd = remoteCLICommandWithFlags("set-environment", "set-environment [flags] VAR VALUE", "Set a session environment variable", remoteCLIFlags{target: true})

var listKeysCmd = remoteCLICommand("list-keys", "list-keys", "List prefix key bindings")

var bindKeyCmd = remoteCLICommand("bind-key", "bind-key KEY ACTION", "Bind a prefix key to an action", cobra.ExactArgs(2))

var listBuffersCmd = remoteCLICommand("list-buffers", "list-buffers", "List paste buffers")

var setBufferCmd = remoteCLICommand("set-buffer", "set-buffer [name] DATA", "Store text in a paste buffer")

var pasteBufferCmd = remoteCLICommandWithFlags("paste-buffer", "paste-buffer", "Paste a buffer into a pane", remoteCLIFlags{target: true})

var resizePaneCmd = remoteCLICommandWithFlags("resize-pane", "resize-pane [flags]", "Resize the active pane", remoteCLIFlags{target: true})

var swapPaneCmd = remoteCLICommandWithFlags("swap-pane", "swap-pane [direction]", "Swap the active pane with a neighbor", remoteCLIFlags{target: true})

var movePaneCmd = remoteCLICommandWithFlags("move-pane", "move-pane [flags]", "Move a pane to another window or break it out", remoteCLIFlags{target: true})

var breakPaneCmd = remoteCLICommandWithFlags("break-pane", "break-pane [flags]", "Break a pane out into a new window", remoteCLIFlags{target: true})

var joinPaneCmd = remoteCLICommandWithFlags("join-pane", "join-pane [flags]", "Join a pane into another window", remoteCLIFlags{target: true})

var selectLayoutCmd = remoteCLICommandWithFlags("select-layout", "select-layout [preset]", "Apply a layout preset to the active window", remoteCLIFlags{target: true})

var chooseTreeCmd = remoteCLICommandWithFlags("choose-tree", "choose-tree", "Open the session/window/pane tree on an attached client", remoteCLIFlags{client: true})

var commandPromptCmd = remoteCLICommandWithFlags("command-prompt", "command-prompt", "Open the command prompt on an attached client", remoteCLIFlags{client: true})

var displayMenuCmd = remoteCLICommand("display-menu", "display-menu", "Open an interactive menu on an attached client")

// ps lists live daemon state (running sessions / panes).
var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List running sessions and panes",
	Long:  "Show live daemon state. Default view lists panes; use --sessions for session names only.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if psSessions {
			return runListSessions(cmd.Context())
		}
		return runListPanes(cmd.Context(), psJSON)
	},
}

// ls lists on-disk resurrection store (manifest + journals).
var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List on-disk checkpoints and journals",
	Long:  "Show persisted resurrection artifacts in state_dir. Works without a running daemon.",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := configStateDir()
		if err != nil {
			return err
		}
		return client.Ls(cmd.Context(), dir, lsJSON)
	},
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove orphan journals from the store",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := configStateDir()
		if err != nil {
			return err
		}
		return client.Prune(cmd.Context(), dir, pruneDryRun, pruneJSON)
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove the on-disk store (manifest and all journals)",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := bindAddr()
		if err != nil {
			return err
		}
		dir, err := configStateDir()
		if err != nil {
			return err
		}
		return client.Rm(cmd.Context(), addr, dir, rmForce, rmJSON)
	},
}

var checkpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "Save a resurrection checkpoint from the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := bindAddr()
		if err != nil {
			return err
		}
		return client.Checkpoint(cmd.Context(), addr, checkpointJSON)
	},
}

var (
	psJSON           bool
	psSessions       bool
	lsJSON           bool
	pruneJSON        bool
	pruneDryRun      bool
	rmJSON           bool
	rmForce          bool
	checkpointJSON   bool
	killSessionTarget string
	cliTarget        string
	cliClient        string
	splitHorizontal  bool
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&bashShell, "bash", false, "use /bin/bash for panes when spawning a new daemon; ignored when attaching to an existing daemon")
	listWindowsCmd.Flags().BoolVar(&listWindowsJSON, "json", false, "print machine-readable JSON")
	listPanesCmd.Flags().BoolVar(&listPanesJSON, "json", false, "print machine-readable JSON")
	displayMessageCmd.Flags().BoolVar(&displayMessageJSON, "json", false, "print machine-readable JSON")
	attachCmd.Flags().BoolVarP(&controlMode, "control", "C", false, "attach in experimental line-oriented control mode")
	attachCmd.Flags().StringVarP(&attachTarget, "target", "t", "", "attach to named session")
	newSessionCmd.Flags().StringVarP(&sessionName, "session", "s", "", "session name")
	_ = newSessionCmd.MarkFlagRequired("session")
	killSessionCmd.Flags().StringVarP(&killSessionTarget, "target", "t", "", "session to kill")
	_ = killSessionCmd.MarkFlagRequired("target")
	hasSessionCmd.Flags().StringVarP(&cliTarget, "target", "t", "", "session name")
	_ = hasSessionCmd.MarkFlagRequired("target")
	for _, c := range []*cobra.Command{newWindowCmd, killWindowCmd, killPaneCmd, selectWindowCmd, splitWindowCmd, sendKeysCmd, capturePaneCmd} {
		c.Flags().StringVarP(&cliTarget, "target", "t", "", "target session:window.pane")
	}
	splitWindowCmd.Flags().BoolVarP(&splitHorizontal, "horizontal", "h", false, "split left/right")
	listWindowsCmd.Flags().StringVarP(&cliTarget, "target", "t", "", "session name")
	listPanesCmd.Flags().StringVarP(&cliTarget, "target", "t", "", "session name")
	displayMessageCmd.Flags().StringVarP(&cliTarget, "target", "t", "", "target session:window.pane")
	renameWindowCmd.Flags().StringVarP(&cliTarget, "target", "t", "", "target session:window")
	renamePaneCmd.Flags().StringVarP(&cliTarget, "target", "t", "", "target session:window.pane")
	psCmd.Flags().BoolVar(&psJSON, "json", false, "print machine-readable JSON")
	psCmd.Flags().BoolVarP(&psSessions, "sessions", "s", false, "list session names only")
	lsCmd.Flags().BoolVar(&lsJSON, "json", false, "print machine-readable JSON")
	pruneCmd.Flags().BoolVar(&pruneJSON, "json", false, "print machine-readable JSON")
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "list orphans without deleting")
	rmCmd.Flags().BoolVar(&rmJSON, "json", false, "print machine-readable JSON")
	rmCmd.Flags().BoolVar(&rmForce, "force", false, "remove store even when the daemon is running")
	checkpointCmd.Flags().BoolVar(&checkpointJSON, "json", false, "print machine-readable JSON")
	registerRemoteCLIFlags(
		switchClientCmd,
		setEnvironmentCmd,
		pasteBufferCmd, resizePaneCmd, swapPaneCmd, movePaneCmd, breakPaneCmd, joinPaneCmd, selectLayoutCmd,
		chooseTreeCmd, commandPromptCmd,
	)
	rootCmd.AddCommand(
		attachCmd, detachCmd, restartCmd, newSessionCmd, killSessionCmd, hasSessionCmd,
		newWindowCmd, killWindowCmd, killPaneCmd, selectWindowCmd, splitWindowCmd,
		sendKeysCmd, capturePaneCmd, listCommandsCmd,
		killServerCmd, sourceFileCmd, listClientsCmd, switchClientCmd,
		showOptionsCmd, setOptionCmd, showEnvironmentCmd, setEnvironmentCmd,
		listKeysCmd, bindKeyCmd, listBuffersCmd, setBufferCmd, pasteBufferCmd,
		resizePaneCmd, swapPaneCmd, movePaneCmd, breakPaneCmd, joinPaneCmd, selectLayoutCmd,
		chooseTreeCmd, commandPromptCmd, displayMenuCmd,
		psCmd, lsCmd, pruneCmd, rmCmd, checkpointCmd,
		listSessionsCmd, listWindowsCmd, listPanesCmd,
		displayMessageCmd, renameWindowCmd, renamePaneCmd,
	)
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

func runKillSession(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.KillSession(ctx, addr, attachOptions(), killSessionTarget)
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
	return client.ListWindowsWithTarget(ctx, addr, jsonOutput, cliTarget)
}

func runListPanes(ctx context.Context, jsonOutput bool) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.ListPanesWithTarget(ctx, addr, jsonOutput, cliTarget)
}

func runDisplayMessage(ctx context.Context, format string, jsonOutput bool) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	if cliTarget != "" {
		args := []string{"display-message", "-t", cliTarget}
		if jsonOutput {
			args = append(args, "--json")
		}
		args = append(args, format)
		out, err := client.RunControlCommand(ctx, addr, args...)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	}
	return client.DisplayMessage(ctx, addr, format, jsonOutput)
}

func runRenameWindow(ctx context.Context, name string) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	args := []string{"rename-window"}
	if cliTarget != "" {
		args = append(args, "-t", cliTarget)
	}
	args = append(args, name)
	out, err := client.RunControlCommand(ctx, addr, args...)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func runRenamePane(ctx context.Context, name string) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	args := []string{"rename-pane"}
	if cliTarget != "" {
		args = append(args, "-t", cliTarget)
	}
	args = append(args, name)
	out, err := client.RunControlCommand(ctx, addr, args...)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func runHasSession(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.HasSession(ctx, addr, attachOptions(), cliTarget)
}

func runNewWindow(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.NewWindow(ctx, addr, cliTarget)
}

func runKillWindow(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.KillWindow(ctx, addr, cliTarget)
}

func runKillPane(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.KillPane(ctx, addr, cliTarget)
}

func runSelectWindow(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.SelectWindow(ctx, addr, cliTarget)
}

func runSplitWindow(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.SplitWindow(ctx, addr, cliTarget, splitHorizontal)
}

func runSendKeys(ctx context.Context, keys []string) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.SendKeys(ctx, addr, cliTarget, keys...)
}

func runCapturePane(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	out, err := client.CapturePane(ctx, addr, cliTarget)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func runListCommands(ctx context.Context) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	return client.ListCommands(ctx, addr)
}

func runRemoteCLI(ctx context.Context, argv ...string) error {
	addr, err := bindAddr()
	if err != nil {
		return err
	}
	out, err := client.RunControlCommand(ctx, addr, argv...)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

type remoteCLIFlags struct {
	target        bool
	requireTarget bool
	client        bool
}

func remoteCLICommand(name, use, short string, args ...cobra.PositionalArgs) *cobra.Command {
	var argCheck cobra.PositionalArgs
	if len(args) > 0 {
		argCheck = args[0]
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  argCheck,
		RunE: func(cmd *cobra.Command, argv []string) error {
			return runRemoteCLI(cmd.Context(), append([]string{name}, argv...)...)
		},
	}
}

func remoteCLICommandWithFlags(name, use, short string, flags remoteCLIFlags, args ...cobra.PositionalArgs) *cobra.Command {
	cmd := remoteCLICommand(name, use, short, args...)
	cmd.RunE = func(c *cobra.Command, argv []string) error {
		remoteArgs := []string{name}
		if flags.client && cliClient != "" {
			remoteArgs = append(remoteArgs, "-c", cliClient)
		}
		if flags.target && cliTarget != "" {
			remoteArgs = append(remoteArgs, "-t", cliTarget)
		}
		return runRemoteCLI(c.Context(), append(remoteArgs, argv...)...)
	}
	return cmd
}

func registerRemoteCLIFlags(cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		switch cmd {
		case switchClientCmd:
			cmd.Flags().StringVarP(&cliTarget, "target", "t", "", "session to attach client to")
			_ = cmd.MarkFlagRequired("target")
			cmd.Flags().StringVarP(&cliClient, "client", "c", "", "client to switch")
		case chooseTreeCmd, commandPromptCmd:
			cmd.Flags().StringVarP(&cliClient, "client", "c", "", "client to target")
		default:
			cmd.Flags().StringVarP(&cliTarget, "target", "t", "", "target session:window.pane")
		}
	}
}

func attachOptions() client.AttachOptions {
	return client.AttachOptions{Bash: bashShell, Control: controlMode, TargetSession: attachTarget}
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
