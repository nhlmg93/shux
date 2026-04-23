package main

type Window struct {
	id         uint32
}

func NewWindow(id uint32) (*Window, error) {
	return &Window{
		id:    id,
	}, nil
}
