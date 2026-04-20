package shux

import "strings"

const (
	DefaultShell       = "/bin/sh"
	DefaultSessionName = "default"
)

func normalizeShell(shell string) string {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		return DefaultShell
	}
	return shell
}

func normalizeSessionName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultSessionName
	}
	return name
}
