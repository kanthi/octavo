// Package ghpages publishes a book to a git branch that GitHub Pages serves.
package ghpages

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/fsutil"
	"github.com/kanthi/octavo/internal/publish"
)

type Settings struct {
	Remote string `yaml:"remote"` // a remote name in the book's repository, or a repository URL
	Branch string `yaml:"branch"`
	CNAME  string `yaml:"cname"` // custom domain, optional
}

type publisher struct{ s Settings }

func New(node yaml.Node) (publish.Publisher, error) {
	s := Settings{Remote: "origin", Branch: "gh-pages"}
	if err := config.Decode(node, &s); err != nil {
		return nil, err
	}
	return &publisher{s}, nil
}

// Publish replaces the branch with one commit that holds the rendered book.
// The branch is force-pushed, so it keeps no history.
func (p *publisher) Publish(ctx context.Context, site publish.Site) error {
	url := p.s.Remote
	if !strings.ContainsAny(url, ":/") { // a remote name, not a URL or path
		out, err := git(ctx, site.BookDir, "remote", "get-url", url)
		if err != nil {
			return fmt.Errorf("no git remote %q for %s; set \"remote\" to a repository URL", url, site.BookDir)
		}
		url = strings.TrimSpace(out)
	}

	tmp, err := os.MkdirTemp("", "octavo-gh-pages-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if _, err := fsutil.CopyTree(site.Dir, tmp); err != nil {
		return err
	}
	// .nojekyll stops GitHub from running Jekyll over the files.
	if err := os.WriteFile(filepath.Join(tmp, ".nojekyll"), nil, 0o644); err != nil {
		return err
	}
	if p.s.CNAME != "" {
		if err := os.WriteFile(filepath.Join(tmp, "CNAME"), []byte(p.s.CNAME+"\n"), 0o644); err != nil {
			return err
		}
	}

	commit := []string{"-c", "commit.gpgsign=false"}
	if name, _ := git(ctx, tmp, "config", "user.name"); strings.TrimSpace(name) == "" {
		commit = append(commit, "-c", "user.name=octavo", "-c", "user.email=octavo@localhost")
	}
	commit = append(commit, "commit", "-q", "-m", "Publish book with octavo")
	steps := [][]string{
		{"init", "-q"},
		{"symbolic-ref", "HEAD", "refs/heads/" + p.s.Branch},
		{"add", "-A"},
		commit,
		{"push", "-q", "--force", url, "HEAD:refs/heads/" + p.s.Branch},
	}
	for _, args := range steps {
		if _, err := git(ctx, tmp, args...); err != nil {
			return err
		}
	}
	fmt.Fprintf(site.Log, "Pushed to branch %s of %s\n", p.s.Branch, url)
	return nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
