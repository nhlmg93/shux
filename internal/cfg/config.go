package cfg

import "time"

const (
	DefaultShellPath              = "/bin/sh"
	BashShellPath                 = "/bin/bash"
	DefaultBindAddr               = "127.0.0.1:23234"
	DefaultMapLeader              = "ctrl+b"
	DefaultScrollback             = 10_000
	DefaultMaxSessions            = 8
	DefaultJournalMaxMB           = 64
	DefaultJournalReplayDelay     = 200 * time.Millisecond
	DefaultPaneQuickSelectTimeout = 2 * time.Second
)

// Config holds runtime policy chosen when a shux daemon starts.
// It is intentionally immutable after actor startup. Attaching to an
// already-running daemon must not mutate this policy.
type Config struct {
	ShellPath              string
	BindAddr               string
	MapLeader              string
	Scrollback             uint
	MaxSessions            uint
	JournalMaxMB           uint
	JournalReplayDelay     time.Duration
	PaneQuickSelectTimeout time.Duration
	StateDir               string
	Resurrection           bool
	Keymaps                *Keymaps
}

func DefaultConfig() Config {
	return Config{
		ShellPath:              DefaultShellPath,
		BindAddr:               DefaultBindAddr,
		MapLeader:              DefaultMapLeader,
		Scrollback:             DefaultScrollback,
		MaxSessions:            DefaultMaxSessions,
		JournalMaxMB:           DefaultJournalMaxMB,
		JournalReplayDelay:     DefaultJournalReplayDelay,
		PaneQuickSelectTimeout: DefaultPaneQuickSelectTimeout,
		Resurrection:           true,
		Keymaps:                DefaultKeymaps(),
	}
}

func BashConfig() Config {
	c := DefaultConfig()
	c.ShellPath = BashShellPath
	return c
}

func (c Config) WithDefaults() Config {
	if c.ShellPath == "" {
		c.ShellPath = DefaultShellPath
	}
	if c.BindAddr == "" {
		c.BindAddr = DefaultBindAddr
	}
	if c.MapLeader == "" {
		c.MapLeader = DefaultMapLeader
	} else {
		c.MapLeader = NormalizeMapLeader(c.MapLeader)
	}
	if c.Scrollback == 0 {
		c.Scrollback = DefaultScrollback
	}
	if c.MaxSessions == 0 {
		c.MaxSessions = DefaultMaxSessions
	}
	if c.JournalMaxMB == 0 {
		c.JournalMaxMB = DefaultJournalMaxMB
	}
	if c.Keymaps == nil {
		c.Keymaps = DefaultKeymaps()
	}
	if c.PaneQuickSelectTimeout <= 0 {
		c.PaneQuickSelectTimeout = DefaultPaneQuickSelectTimeout
	}
	return c
}
