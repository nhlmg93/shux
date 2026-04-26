package sshkey

import (
	"fmt"
	"os"
	"path/filepath"
)

const hostKeyFile = "ssh_host_ed25519"

func HostKeyPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("sshkey: user config dir: %w", err)
	}
	dir = filepath.Join(dir, "shux")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("sshkey: create config dir: %w", err)
	}
	return filepath.Join(dir, hostKeyFile), nil
}
