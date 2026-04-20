package shux

import "strings"

const (
	DefaultShell       = "/bin/sh"
	DefaultSessionName = "default"
	DefaultRows        = 24
	DefaultCols        = 80
	MaxTermDimension   = 65535
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

func sanitizeTermSize(rows, cols int) (int, int, bool) {
	originalRows, originalCols := rows, cols

	if rows <= 0 {
		rows = DefaultRows
	} else if rows > MaxTermDimension {
		rows = MaxTermDimension
	}

	if cols <= 0 {
		cols = DefaultCols
	} else if cols > MaxTermDimension {
		cols = MaxTermDimension
	}

	return rows, cols, rows != originalRows || cols != originalCols
}
