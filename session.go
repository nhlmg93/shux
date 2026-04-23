package main

type Session struct {
	id         uint32
}

func NewSession(id uint32) (*Session, error) {
	return &Session{
		id:    id,
	}, nil
}
