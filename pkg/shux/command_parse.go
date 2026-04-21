package shux

import (
	"fmt"
	"strings"
)

// ParseCommand parses a command string into a Command.
// Commands follow tmux-like syntax: "command-name arg1 arg2 ..."
func ParseCommand(input string) (Command, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Command{}, fmt.Errorf("empty command")
	}

	fields := tokenizeCommand(input)
	if len(fields) == 0 {
		return Command{}, fmt.Errorf("empty command")
	}

	return Command{
		Name: fields[0],
		Args: fields[1:],
	}, nil
}

// tokenizeCommand splits input into fields, handling basic quoting.
func tokenizeCommand(input string) []string {
	var fields []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range input {
		switch {
		case !inQuote && (r == '"' || r == '\''):
			inQuote = true
			quoteChar = r
		case inQuote && r == quoteChar:
			inQuote = false
			quoteChar = 0
		case !inQuote && (r == ' ' || r == '\t'):
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		fields = append(fields, current.String())
	}

	return fields
}
