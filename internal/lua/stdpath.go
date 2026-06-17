package lua

import (
	"os"
	"path/filepath"
)

// Stdpath returns XDG-style paths for shux, mirroring Neovim's stdpath().
func Stdpath(name string) (string, error) {
	switch name {
	case "config":
		if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
			return filepath.Join(dir, "shux"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "shux"), nil
	case "data":
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			return filepath.Join(dir, "shux"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "shux"), nil
	case "state":
		if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
			return filepath.Join(dir, "shux"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "state", "shux"), nil
	case "cache":
		if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
			return filepath.Join(dir, "shux"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cache", "shux"), nil
	case "runtime":
		config, err := Stdpath("config")
		if err != nil {
			return "", err
		}
		return config, nil
	default:
		return "", os.ErrInvalid
	}
}
