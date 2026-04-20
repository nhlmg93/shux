package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

func newRootCmd() *cobra.Command {
	var cli cliOptions

	cmd := &cobra.Command{
		Use:           "shux [session]",
		Short:         "A terminal multiplexer that just works",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := resolveRunOptions(args, cli)
			if err != nil {
				return err
			}
			return runApp(opts)
		},
	}

	cmd.Flags().StringVar(&cli.ConfigPath, "config", "", "Load config from an alternate Lua file path")
	cmd.Flags().StringVarP(&cli.SessionName, "session", "s", "", "Session name")
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print shux version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version)
			return err
		},
	}
}
