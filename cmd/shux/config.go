package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"shux/pkg/shux"
)

const defaultSessionName = "default"

var userConfigDir = os.UserConfigDir

type Config struct {
	Session struct {
		Name string
	}
	Shell string
	Mouse *bool
	Keys  shux.KeymapConfig
}

type RunOptions struct {
	SessionName  string
	Shell        string
	MouseEnabled bool
	Keymap       shux.Keymap
}

type cliOptions struct {
	ConfigPath  string
	SessionName string
}

func resolveRunOptions(args []string, cli cliOptions) (RunOptions, error) {
	configPath, explicit, err := resolveConfigPath(cli.ConfigPath)
	if err != nil {
		return RunOptions{}, err
	}

	cfg, err := loadConfig(configPath, explicit)
	if err != nil {
		return RunOptions{}, err
	}

	sessionName := strings.TrimSpace(cfg.Session.Name)
	if cli.SessionName != "" {
		sessionName = strings.TrimSpace(cli.SessionName)
	}
	if len(args) > 0 {
		sessionName = strings.TrimSpace(args[0])
	}
	if sessionName == "" {
		sessionName = defaultSessionName
	}

	shell := strings.TrimSpace(cfg.Shell)
	if shell == "" {
		shell = defaultShell()
	}

	keymap, err := shux.NewKeymap(cfg.Keys)
	if err != nil {
		return RunOptions{}, fmt.Errorf("resolve keymap: %w", err)
	}

	mouseEnabled := true
	if cfg.Mouse != nil {
		mouseEnabled = *cfg.Mouse
	}

	return RunOptions{
		SessionName:  sessionName,
		Shell:        shell,
		MouseEnabled: mouseEnabled,
		Keymap:       keymap,
	}, nil
}

func resolveConfigPath(cliPath string) (string, bool, error) {
	if cliPath != "" {
		return cliPath, true, nil
	}
	path, err := defaultConfigPath()
	if err != nil {
		return "", false, fmt.Errorf("resolve default config path: %w", err)
	}
	return path, false, nil
}

func defaultConfigPath() (string, error) {
	configDir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "shux", "init.lua"), nil
}

func defaultShell() string {
	if shell := strings.TrimSpace(os.Getenv("SHELL")); shell != "" {
		return shell
	}
	return shux.DefaultShell
}
