package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:     "shux",
	Short:   "shux / \"you shouldn't have\" /",
	Version: "0.1.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		shux, err := NewShux()
		if err != nil {
			return err
		}
		return shux.Run()

	},
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
