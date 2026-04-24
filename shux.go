package main

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"shux-dev/internal/supervisor"
	"shux-dev/internal/ui"
)

type Shux struct {
	Logger *Logger
}

func NewShux() (*Shux, error) {
	logger, err := NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	return &Shux{
		Logger: logger,
	}, nil
}

func (a *Shux) Run() error {
	defer a.Logger.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = supervisor.Start(ctx)

	_, err := tea.NewProgram(ui.NewModel()).Run()
	if err != nil {
		return fmt.Errorf("failed to run ui: %w", err)
	}

	return nil
}
