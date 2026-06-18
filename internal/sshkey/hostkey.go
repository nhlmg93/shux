package sshkey

import (
	"fmt"
	"os"
	"path/filepath"
)

const hostKeyFile = "ssh_host_ed25519"

func HostKeyPath() (string, error) {
	// Host keys are daemon identity, not user Lua config. Always use the real
	// ~/.config/shux path so CLI clients match daemons even when XDG_CONFIG_HOME
	// is overridden (tests, CI, agent shells).
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("sshkey: home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "shux")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("sshkey: create config dir: %w", err)
	}
	return filepath.Join(dir, hostKeyFile), nil
}
