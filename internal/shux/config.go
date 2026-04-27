package shux

const (
	DefaultShellPath = "/bin/sh"
	BashShellPath    = "/bin/bash"
)

// Config holds runtime policy chosen when a shux daemon starts.
// It is intentionally small and immutable after actor startup. Attaching to an
// already-running daemon must not mutate this policy.
type Config struct {
	ShellPath string
}

func DefaultConfig() Config {
	return Config{ShellPath: DefaultShellPath}
}

func BashConfig() Config {
	return Config{ShellPath: BashShellPath}
}

func (c Config) WithDefaults() Config {
	if c.ShellPath == "" {
		c.ShellPath = DefaultShellPath
	}
	return c
}
