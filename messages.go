package main

// Message types for actor communication

type CreatePane struct {
	Cmd  string
	Args []string
}

type KillPane struct{}

type PaneExited struct {
	ID uint32
}

type SwitchToPane struct {
	Index int
}

type PaneOutput struct {
	ID   uint32
	Data []byte
}

type WriteToPane struct {
	Data []byte
}

type CreateWindow struct{}

type WindowEmpty struct {
	ID uint32
}

type SwitchWindow struct {
	Delta int
}

type GetActivePane struct{}

type GetActiveWindow struct{}
