package main

import (
	"github.com/spf13/cobra"
	"shux/internal/shux"
)

var rootCmd = &cobra.Command{
	Use:     "shux",
	Short:   "shux / \"you shouldn't have\" /",
	Version: "0.1.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := shux.NewShux()
		if err != nil {
			return err
		}
		return s.Run()

	},
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
