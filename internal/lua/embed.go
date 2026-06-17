package lua

import (
	"embed"
	"io/fs"
	"path/filepath"
	"sort"
)

//go:embed all:runtime
var embeddedRuntime embed.FS

func embeddedRuntimeDir(sub string) fs.FS {
	sub = filepath.ToSlash(sub)
	root, err := fs.Sub(embeddedRuntime, filepath.Join("runtime", sub))
	if err != nil {
		panic("lua: embedded runtime: " + err.Error())
	}
	return root
}

func listLuaFiles(root fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(root, dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		if filepath.Ext(name) == ".lua" {
			out = append(out, filepath.Join(dir, name))
		}
	}
	sort.Strings(out)
	return out, nil
}
