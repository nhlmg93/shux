package cfg

import (
	"strings"
)

// NormalizeMapLeader converts Neovim-style leader notation to Bubble Tea key names.
// Example: "<C-b>" -> "ctrl+b".
func NormalizeMapLeader(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return DefaultMapLeader
	}
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		inner := strings.Trim(s, "<>")
		mod, key, ok := strings.Cut(inner, "-")
		if ok {
			switch strings.ToLower(mod) {
			case "c", "ctrl", "control":
				return "ctrl+" + strings.ToLower(key)
			case "m", "meta", "alt":
				return "alt+" + strings.ToLower(key)
			case "s", "shift":
				return "shift+" + strings.ToLower(key)
			}
		}
	}
	return strings.ToLower(s)
}
