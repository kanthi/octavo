// Package guide holds the Octavo Guide: the user guide, written as an
// ordinary Octavo book in this folder and built into the binary, so that
// "octavo guide" always matches the version being run.
//
// To work on it: octavo preview --dir guide
package guide

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

//go:embed octavo.yml index.md content images styles
var files embed.FS

// Placeholder in the guide's text that is replaced with the running version.
const Placeholder = "{{octavo-version}}"

// WriteTo writes the guide's source into dir, which must not exist or must be
// empty, so that nothing is ever overwritten. Placeholder is replaced with
// version in Markdown and YAML files.
func WriteTo(dir, version string) error {
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s already exists and is not empty; choose a new folder", dir)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := files.ReadFile(p)
		if err != nil {
			return err
		}
		if ext := path.Ext(p); ext == ".md" || ext == ".yml" {
			data = []byte(strings.ReplaceAll(string(data), Placeholder, version))
		}
		target := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
