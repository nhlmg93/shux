package main 

import "fmt"

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
