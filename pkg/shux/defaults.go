package shux

import "strings"

const DefaultShell = "/bin/sh"

func normalizeShell(shell string) string {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		return DefaultShell
	}
	return shell
}
