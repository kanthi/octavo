// Package versions builds a book in several versions side by side, such as
// a 2nd and a 1st edition, each in its own folder of the output:
//
//	_book/
//	├── index.html, chapters/...   forward to the latest version
//	├── 2nd-edition/               the book as it is now
//	└── 1st-edition/               the book as it was at a git tag
//
// The versions are listed under "versions:" in octavo.yml; the first is the
// latest. A version with a "ref:" is built from that git branch, tag or
// commit, checked out into a temporary folder with "git worktree", so the
// book's working folder is never touched. A book without versions builds as
// usual, straight into the output folder.
package versions

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/format"
	"github.com/kanthi/octavo/internal/format/html"
	"github.com/kanthi/octavo/internal/fsutil"
)

// Latest returns the folder of the latest version, or "" when the book has
// no versions.
func Latest(b *book.Book) string {
	if len(b.Config.Versions) == 0 {
		return ""
	}
	return b.Config.Versions[0].Folder
}

// Build renders the book into dir: every version when it has versions,
// else just the book. It returns the files written, relative to dir.
func Build(b *book.Book, dir string, opts format.Options) ([]string, error) {
	if len(b.Config.Versions) == 0 {
		out := format.NewOutput(dir)
		err := format.RenderAll(b, out, opts)
		return out.Changed(), err
	}
	if !format.Partial(opts.Formats) {
		if err := fsutil.EmptyDir(dir); err != nil {
			return nil, err
		}
	}
	var changed []string
	for i, v := range b.Config.Versions {
		vb, done, err := checkout(b, v)
		if err != nil {
			return nil, fmt.Errorf("version %s: %w", v.Label, err)
		}
		files := format.NewOutput(filepath.Join(dir, v.Folder))
		err = format.RenderAll(vb, files, versionOptions(b.Config.Versions, i, opts))
		done()
		if err != nil {
			return nil, fmt.Errorf("version %s: %w", v.Label, err)
		}
		for _, f := range files.Changed() {
			changed = append(changed, v.Folder+"/"+f)
		}
	}
	more, err := writeRoot(b, dir, opts)
	return append(changed, more...), err
}

// versionOptions are the options for building version i: its own address,
// links one folder further up, and the version menu.
func versionOptions(all []config.Version, i int, opts format.Options) format.Options {
	v := all[i]
	o := opts
	o.InSite = true // the root of the output gets the 404 page and the sitemap
	if o.SiteURL != "" {
		o.SiteURL = strings.TrimSuffix(o.SiteURL, "/") + "/" + v.Folder + "/"
	}
	if o.HomeURL != "" && !strings.Contains(o.HomeURL, "://") {
		o.HomeURL = "../" + o.HomeURL
	}
	// A library moves the latest version's PDF and EPUB beside the other
	// books' (see the library package); older versions keep theirs.
	if opts.DownloadURL != nil {
		if i == 0 {
			o.DownloadURL = func(kind, file string) string { return "../" + opts.DownloadURL(kind, file) }
		} else {
			o.DownloadURL = nil
		}
	}
	o.Versions = nil
	for j, other := range all {
		o.Versions = append(o.Versions, format.Version{
			Label: other.Label, URL: "../" + other.Folder + "/", Current: j == i, Latest: j == 0,
		})
	}
	return o
}

// checkout returns the book as it is in version v, and a function that
// removes the temporary checkout.
func checkout(b *book.Book, v config.Version) (*book.Book, func(), error) {
	if v.Ref == "" {
		return b, func() {}, nil
	}
	top, err := git(b.Dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, nil, fmt.Errorf("ref %q needs the book to be in a git repository", v.Ref)
	}
	if _, err := git(b.Dir, "rev-parse", "--verify", "--quiet", v.Ref+"^{commit}"); err != nil {
		return nil, nil, fmt.Errorf("git has no branch, tag or commit %q (fetch it first, or fix \"ref:\")", v.Ref)
	}
	rel, err := filepath.Rel(resolve(top), resolve(b.Dir))
	if err != nil || !filepath.IsLocal(rel) && rel != "." {
		return nil, nil, fmt.Errorf("the book folder is outside its git repository %s", top)
	}
	tmp, err := os.MkdirTemp("", "octavo-version-")
	if err != nil {
		return nil, nil, err
	}
	tree := filepath.Join(tmp, "tree")
	done := func() {
		git(top, "worktree", "remove", "--force", tree)
		os.RemoveAll(tmp)
		git(top, "worktree", "prune")
	}
	if _, err := git(top, "worktree", "add", "--detach", "--quiet", tree, v.Ref); err != nil {
		done()
		return nil, nil, err
	}
	vb, err := book.Load(filepath.Join(tree, rel))
	if err != nil {
		done()
		return nil, nil, fmt.Errorf("at %s: %w", v.Ref, err)
	}
	// The version is built into the book's own output folder, not into the
	// temporary checkout's.
	return vb, done, nil
}

// writeRoot writes the files at the top of the output: every page of the
// latest version forwards there from its address without a version, so links
// made before the book had versions keep working, and always lead to the
// latest. Plus the 404 page and the sitemap.
func writeRoot(b *book.Book, dir string, opts format.Options) ([]string, error) {
	latest := Latest(b)
	var pages, changed []string
	err := filepath.WalkDir(filepath.Join(dir, latest), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".html" || d.Name() == "404.html" {
			return err
		}
		rel, _ := filepath.Rel(filepath.Join(dir, latest), p)
		rel = filepath.ToSlash(rel)
		if !html.IsRedirect(p) {
			pages = append(pages, latest+"/"+rel)
		}
		up := strings.Repeat("../", strings.Count(rel, "/"))
		if err := fsutil.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), html.RedirectPage(b.Config.Book.Title, up+latest+"/"+rel)); err != nil {
			return err
		}
		changed = append(changed, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if opts.InSite {
		return changed, nil // a library writes the site's own sitemap and 404 page
	}
	siteURL := opts.SiteURL
	if siteURL == "" {
		siteURL = b.Config.Book.SiteURL
	}
	if siteURL != "" {
		siteURL = strings.TrimSuffix(siteURL, "/") + "/"
		if err := fsutil.WriteFile(filepath.Join(dir, "sitemap.xml"), html.Sitemap(siteURL, pages)); err != nil {
			return nil, err
		}
		changed = append(changed, "sitemap.xml")
	}
	if err := fsutil.WriteFile(filepath.Join(dir, "404.html"), html.NotFound(b.Config.Book.Title, siteURL)); err != nil {
		return nil, err
	}
	return append(changed, "404.html"), nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// resolve follows symbolic links, as git does (on macOS, /var is /private/var).
func resolve(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
