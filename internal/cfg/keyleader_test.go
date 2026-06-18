package cfg_test

import (
	"testing"

	"shux/internal/cfg"
)

func TestNormalizeMapLeader_neovimStyle(t *testing.T) {
	tests := map[string]string{
		"<C-b>":     "ctrl+b",
		"<C-a>":     "ctrl+a",
		"<M-x>":     "alt+x",
		"ctrl+b":    "ctrl+b",
		"  <C-c>  ": "ctrl+c",
	}
	for in, want := range tests {
		if got := cfg.NormalizeMapLeader(in); got != want {
			t.Fatalf("NormalizeMapLeader(%q) = %q, want %q", in, got, want)
		}
	}
}
