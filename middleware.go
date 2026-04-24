package main

import (
	"charm.land/wish/v2"
	"github.com/charmbracelet/ssh"
)

func ShuxUiMiddleware(_ *Logger) wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return next
	}
}
