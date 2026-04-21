package shux

import (
	"testing"
	"time"
)

func TestEnsureTermEnvAddsDefaultTERM(t *testing.T) {
	env := ensureTermEnv([]string{"HOME=/tmp", "SHELL=/bin/sh"})
	if len(env) != 3 {
		t.Fatalf("len(env) = %d, want 3", len(env))
	}
	if env[2] != "TERM=xterm-256color" {
		t.Fatalf("env[2] = %q, want TERM=xterm-256color", env[2])
	}
}

func TestEnsureTermEnvPreservesExistingTERM(t *testing.T) {
	input := []string{"HOME=/tmp", "TERM=screen-256color"}
	env := ensureTermEnv(input)
	if len(env) != len(input) {
		t.Fatalf("len(env) = %d, want %d", len(env), len(input))
	}
	if env[1] != "TERM=screen-256color" {
		t.Fatalf("TERM entry = %q, want TERM=screen-256color", env[1])
	}
}

func TestPaneContentCacheStoreInvalidateAndSchedule(t *testing.T) {
	var cache paneContentCache

	if _, ok := cache.Current(); ok {
		t.Fatal("Current() should miss before content is stored")
	}

	content := &PaneContent{Title: "hello"}
	if got := cache.Store(content); got != content {
		t.Fatal("Store() should return the stored content")
	}
	if got, ok := cache.Current(); !ok || got != content {
		t.Fatal("Current() should return cached content after Store()")
	}

	cache.Invalidate()
	if _, ok := cache.Current(); ok {
		t.Fatal("Current() should miss after Invalidate()")
	}

	ref := &PaneRef{loopRef: newLoopRef(1)}
	cache.Schedule(ref, 0)
	cache.Schedule(ref, 0)

	time.Sleep(10 * time.Millisecond)

	count := 0
	for {
		select {
		case msg := <-ref.inbox:
			if _, ok := msg.(paneFlushUpdate); !ok {
				t.Fatalf("scheduled message = %T, want paneFlushUpdate", msg)
			}
			count++
		default:
			if count != 1 {
				t.Fatalf("scheduled paneFlushUpdate count = %d, want 1", count)
			}
			cache.ClearPending()
			if cache.updatePending {
				t.Fatal("ClearPending() should reset updatePending")
			}
			return
		}
	}
}
