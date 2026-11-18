// Package command publishes a book by running any shell command. It covers
// hosts that have their own upload tool (Cloudflare Pages, Netlify, S3, ...)
// until they get a dedicated module.
package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/publish"
)

type Settings struct {
	// Run is a shell command. "{dir}" is replaced with the rendered book's
	// folder, which is also available as $OCTAVO_SITE_DIR.
	Run string `yaml:"run"`
}

type publisher struct{ s Settings }

func New(node yaml.Node) (publish.Publisher, error) {
	var s Settings
	if err := config.Decode(node, &s); err != nil {
		return nil, err
	}
	if strings.TrimSpace(s.Run) == "" {
		return nil, fmt.Errorf("run is required, for example: wrangler pages deploy {dir}")
	}
	return &publisher{s}, nil
}

func (p *publisher) Publish(ctx context.Context, site publish.Site) error {
	quoted := "'" + strings.ReplaceAll(site.Dir, "'", `'\''`) + "'"
	cmd := exec.CommandContext(ctx, "sh", "-c", strings.ReplaceAll(p.s.Run, "{dir}", quoted))
	cmd.Dir = site.BookDir
	cmd.Env = append(os.Environ(), "OCTAVO_SITE_DIR="+site.Dir)
	cmd.Stdout, cmd.Stderr = site.Log, site.Log
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%q: %w", p.s.Run, err)
	}
	return nil
}
