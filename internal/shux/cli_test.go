package shux

import (
	"context"
	"testing"
	"time"
)

func TestParseTargetFlag(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	target, rest, err := ParseTargetFlag([]string{"-t", "main:2", "extra"})
	if err != nil {
		t.Fatal(err)
	}
	if target != "main:2" || len(rest) != 1 || rest[0] != "extra" {
		t.Fatalf("ParseTargetFlag = %q %v", target, rest)
	}
}

func TestParseSendKeyToken(t *testing.T) {
	key, text, ok := parseSendKeyToken("Enter")
	if !ok || key != "enter" || text != "\r" {
		t.Fatalf("Enter token = %q %q %v", key, text, ok)
	}
	_, _, ok = parseSendKeyToken("hello")
	if ok {
		t.Fatal("literal should not be key token")
	}
}
