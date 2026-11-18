// Package rsync publishes a book to your own server with rsync over SSH.
package rsync

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/publish"
)

type Settings struct {
	Target string `yaml:"target"` // user@host:/var/www/book
	Delete bool   `yaml:"delete"` // also remove server files that are not in the book
}

type publisher struct{ s Settings }

func New(node yaml.Node) (publish.Publisher, error) {
	var s Settings
	if err := config.Decode(node, &s); err != nil {
		return nil, err
	}
	if s.Target == "" {
		return nil, fmt.Errorf("target is required, for example user@host:/var/www/book")
	}
	return &publisher{s}, nil
}

func (p *publisher) Publish(ctx context.Context, site publish.Site) error {
	args := []string{"-az"}
	if p.s.Delete {
		args = append(args, "--delete")
	}
	// The trailing separator makes rsync copy the folder's contents.
	args = append(args, site.Dir+string(filepath.Separator), p.s.Target)
	cmd := exec.CommandContext(ctx, "rsync", args...)
	cmd.Stdout, cmd.Stderr = site.Log, site.Log
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync: %w", err)
	}
	fmt.Fprintf(site.Log, "Synced to %s\n", p.s.Target)
	return nil
}
