// Package folder publishes a book by copying it into a folder on this
// machine, such as a web server's document root or a mounted share.
package folder

import (
	"context"
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/fsutil"
	"github.com/kanthi/octavo/internal/publish"
)

type Settings struct {
	Path string `yaml:"path"` // absolute, or relative to the book folder
}

type publisher struct{ s Settings }

func New(node yaml.Node) (publish.Publisher, error) {
	var s Settings
	if err := config.Decode(node, &s); err != nil {
		return nil, err
	}
	if s.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	return &publisher{s}, nil
}

// Publish copies every file, overwriting what is there. It does not delete
// files in the folder that are no longer part of the book.
func (p *publisher) Publish(_ context.Context, site publish.Site) error {
	target := p.s.Path
	if !filepath.IsAbs(target) {
		target = filepath.Join(site.BookDir, target)
	}
	n, err := fsutil.CopyTree(site.Dir, target)
	if err != nil {
		return err
	}
	fmt.Fprintf(site.Log, "Copied %d files to %s\n", n, target)
	return nil
}
