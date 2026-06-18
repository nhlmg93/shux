package shux

import (
	"os"
	"path/filepath"

	"shux/internal/lua"
)

func resolveLuaConfigPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	configDir, err := lua.Stdpath("config")
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(configDir, path)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	if _, err := os.Stat(path); err == nil {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	return candidate, nil
}
