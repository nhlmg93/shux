package main

type Pane struct {
	id         uint32
}

func NewPane(id uint32) (*Pane, error) {
	return &Pane{
		id:    id,
	}, nil
}
