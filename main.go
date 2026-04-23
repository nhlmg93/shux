package main

import (
	"fmt"
	"os"
	"github.com/spf13/cobra"
)

func main() {
	logger := NewLogger()

	if err := logger.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

   	cobra.CheckErr(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   "shux",
	Short: "shux / \"you shouldn't have\" /",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}
