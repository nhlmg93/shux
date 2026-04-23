package main

import (
	"fmt"

	"github.com/spf13/cobra"
)  

type Shux struct {
	Logger *Logger
}

func NewShux() (*Shux, error){
	var logger = NewLogger()
	if err := logger.Init(); err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}
	return &Shux{
		Logger: logger,
	}, nil
}

func (a *Shux) Run() error {
	return nil
}

var rootCmd = &cobra.Command{
	Use:   "shux",
	Short: "shux / \"you shouldn't have\" /",
	Version:"0.1.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		shux, err := NewShux()
		if err != nil {
			return err
		}
		return shux.Run()

	},
}


func init() {

}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}

