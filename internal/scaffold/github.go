package scaffold

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kanthi/octavo/internal/fsutil"
)

// GitHub describes the workflow file that publishes a book or library to
// GitHub Pages from GitHub Actions.
type GitHub struct {
	Repo     string // the git repository's top folder
	Workflow string // the workflow file, inside Repo
	Branch   string // pushes to this branch publish
	Owner    string // from the "origin" remote, when it is on GitHub
	Name     string
}

// WorkflowPath is where the workflow goes, relative to the repository.
const WorkflowPath = ".github/workflows/octavo.yml"

// GitHubWorkflow writes the workflow for the book or library in dir, whose
// built site ends up in output. It never overwrites an existing workflow.
func GitHubWorkflow(dir, output string) (*GitHub, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git is not installed; GitHub publishing needs the book in a git repository")
	}
	top, err := gitOut(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("%s is not in a git repository; create one first:\n  git init\n  git remote add origin https://github.com/<you>/<repository>.git", dir)
	}
	g := &GitHub{Repo: top, Branch: "main"}
	g.Workflow = filepath.Join(top, filepath.FromSlash(WorkflowPath))
	if _, err := os.Lstat(g.Workflow); err == nil {
		return nil, fmt.Errorf("%s already exists; Octavo never overwrites it. Delete it first to write a new one", g.Workflow)
	}
	if b, err := gitOut(dir, "symbolic-ref", "--short", "HEAD"); err == nil && b != "" {
		g.Branch = b
	}
	if url, err := gitOut(dir, "remote", "get-url", "origin"); err == nil {
		if m := githubRemote.FindStringSubmatch(url); m != nil {
			g.Owner, g.Name = m[1], strings.TrimSuffix(m[2], ".git")
		}
	}

	// Paths in the workflow are relative to the repository's top folder.
	relTo := func(p string) (string, error) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		abs = resolve(abs)
		rel, err := filepath.Rel(top, abs)
		if err != nil || !filepath.IsLocal(rel) && rel != "." {
			return "", fmt.Errorf("%s is outside the git repository %s", p, top)
		}
		return filepath.ToSlash(rel), nil
	}
	bookRel, err := relTo(dir)
	if err != nil {
		return nil, err
	}
	outRel, err := relTo(output)
	if err != nil {
		return nil, err
	}
	if err := fsutil.WriteFile(g.Workflow, []byte(workflow(g.Branch, bookRel, outRel))); err != nil {
		return nil, err
	}
	return g, nil
}

var githubRemote = regexp.MustCompile(`github\.com[:/]([^/]+)/([^/\s]+?)/?$`)

func gitOut(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// workflow is the GitHub Actions file. It builds with the newest Octavo,
// with Typst for fast PDFs (Chrome, which GitHub's machines have, would do
// too), and hands the site to GitHub Pages without a gh-pages branch.
func workflow(branch, book, output string) string {
	dirFlag := ""
	if book != "." {
		dirFlag = " --dir " + book
	}
	return fmt.Sprintf(`# Builds the book with Octavo and publishes it on GitHub Pages, on every
# push to %[1]s. Written by "octavo publish --setup github"; edit it freely.
#
# Once, on GitHub: Settings > Pages > Build and deployment > Source:
# "GitHub Actions".
name: Publish with Octavo

on:
  push:
    branches: [%[1]s]
  workflow_dispatch: # a "Run workflow" button on the Actions tab

permissions:
  contents: read
  pages: write
  id-token: write

# One publish at a time; a newer push replaces one that is still waiting.
concurrency:
  group: pages
  cancel-in-progress: false

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0 # all history: "Last updated" dates and book versions come from git
      - uses: actions/setup-go@v7
        with:
          go-version: stable
          cache: false
      - name: Install Octavo
        run: go install github.com/kanthi/octavo/cmd/octavo@latest
      - name: Install Typst, for fast PDFs
        uses: typst-community/setup-typst@v5
      - id: pages
        uses: actions/configure-pages@v6
      - name: Build
        # GitHub gives the address as http:// when the repository does not
        # enforce HTTPS, though the site is served over https://.
        run: |
          export OCTAVO_SITE_URL="${BASE_URL/#http:/https:}/"
          octavo render%[2]s
        env:
          BASE_URL: ${{ steps.pages.outputs.base_url }}
      - uses: actions/upload-pages-artifact@v5
        with:
          path: %[3]s

  publish:
    needs: build
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deploy.outputs.page_url }}
    steps:
      - id: deploy
        uses: actions/deploy-pages@v5
`, branch, dirFlag, output)
}

// resolve follows symbolic links in p, as git does for the repository's
// folder (on macOS, /var is /private/var). Folders that do not exist yet,
// such as an output folder before the first build, are kept as they are.
func resolve(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	return filepath.Join(resolve(parent), filepath.Base(p))
}
