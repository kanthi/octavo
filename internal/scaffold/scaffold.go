// Package scaffold creates a new book folder from the embedded sample book.
package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/fsutil"
)

//go:embed skeleton
var skeleton embed.FS

// New writes the sample book into dir. It refuses to touch a folder that
// already holds a book, and never overwrites an existing file.
func New(dir string) error {
	return newBook(dir, false)
}

// NewInLibrary creates a book inside a library's books folder. dir must not
// exist yet, so no existing book or file can be touched. The book's settings
// leave out "publish:", which a library sets once for all its books.
func NewInLibrary(dir string) error {
	if _, err := os.Lstat(dir); err == nil {
		return fmt.Errorf("%s already exists; Octavo never overwrites a book", dir)
	}
	if err := newBook(dir, true); err != nil {
		os.RemoveAll(dir) // only what this call created: dir did not exist before
		return err
	}
	return nil
}

// publishHelp starts the commented publish section of the starter octavo.yml.
const publishHelp = "# Where \"octavo publish\" sends the book."

const libraryPublishNote = "# This book is part of a library. \"octavo publish\" sends the whole library;\n# set it up in octavo-library.yml.\n"

func newBook(dir string, inLibrary bool) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(abs, config.FileName)); err == nil {
		return fmt.Errorf("%s already holds a book (%s exists)", dir, config.FileName)
	}
	replace := strings.NewReplacer(
		"{{title}}", titleFrom(filepath.Base(abs)),
		"{{date}}", time.Now().Format("2006-01-02"),
	)
	return fs.WalkDir(skeleton, "skeleton", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, "skeleton/")
		if rel == "gitignore" { // stored without the dot so it does not act on this repository
			if inLibrary {
				return nil // the library's own .gitignore covers every book
			}
			rel = ".gitignore"
		}
		target := filepath.Join(abs, filepath.FromSlash(rel))
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		data, err := skeleton.ReadFile(p)
		if err != nil {
			return err
		}
		text := replace.Replace(string(data))
		if inLibrary && rel == config.FileName {
			if i := strings.Index(text, publishHelp); i >= 0 {
				text = text[:i] + libraryPublishNote
			}
		}
		return fsutil.WriteFile(target, []byte(text))
	})
}

// CopyBook copies the book in src to dst, which must not exist yet. The
// source is only read. Build output (the book's output-dir), version-control
// folders and Octavo's own cache are left behind.
func CopyBook(src, dst string) (int, error) {
	src, err := filepath.Abs(src)
	if err != nil {
		return 0, err
	}
	dst, err = filepath.Abs(dst)
	if err != nil {
		return 0, err
	}
	cfg, err := config.Load(src)
	if err != nil {
		return 0, err
	}
	if _, err := os.Lstat(dst); err == nil {
		return 0, fmt.Errorf("%s already exists; Octavo never overwrites a book", dst)
	}
	if rel, err := filepath.Rel(src, dst); err == nil && filepath.IsLocal(rel) {
		return 0, fmt.Errorf("%s is inside the book being copied (%s); put the library somewhere else", dst, src)
	}
	skip := map[string]bool{".git": true, ".hg": true, ".svn": true, ".octavo": true}
	output := filepath.Clean(cfg.OutputDir)

	n := 0
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil || rel == "." {
			return err
		}
		if skip[d.Name()] || rel == output {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			n++
			return os.Symlink(link, target)
		case !d.Type().IsRegular():
			return nil // sockets, devices and the like are not book files
		}
		n++
		return fsutil.CopyFile(p, target)
	})
	if err != nil {
		os.RemoveAll(dst) // only what this call created: dst did not exist before
		return 0, err
	}
	return n, nil
}

// ValidBookName checks a name for a book folder in a library. The name is
// also the book's address on the site (html/<Name>/), so it is kept to
// letters, digits, "-", "_" and ".".
func ValidBookName(name string) error {
	if name == "" {
		return fmt.Errorf("a book needs a name")
	}
	for i, r := range name {
		ok := r == '-' || r == '_' || r == '.' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !ok || i == 0 && (r == '.' || r == '-') {
			return fmt.Errorf("book name %q: use letters, digits, \"-\", \"_\" and \".\", starting with a letter or digit", name)
		}
	}
	return nil
}

// titleFrom turns a folder name such as "my-first-book" into "My First Book".
func titleFrom(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	if len(words) == 0 {
		return "My Book"
	}
	return strings.Join(words, " ")
}

const libraryConfig = `library:
  title: "{{title}}"
  description: Books written with Octavo.
  books-dir: books       # every subfolder with a octavo.yml is a book; "octavo add <Name>" adds one
  output-dir: public     # the site to host: the portal plus every book
  theme: auto            # light | dark | auto
  # css: portal.css      # your own styles for the portal page

# Where "octavo publish" sends the public folder. Same targets as a book.
# Or let GitHub build and publish the library on every push:
#   octavo publish --setup github
#
# publish:
#   gh-pages: {}
`

// NewLibrary creates a library folder: its settings file and an empty books
// folder. Add books with "octavo add <Name>".
func NewLibrary(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for _, name := range []string{"octavo-library.yml", config.FileName} {
		if _, err := os.Stat(filepath.Join(abs, name)); err == nil {
			return fmt.Errorf("%s already holds a library or a book (%s exists)", dir, name)
		}
	}
	if err := os.MkdirAll(filepath.Join(abs, "books"), 0o755); err != nil {
		return err
	}
	files := map[string]string{
		"octavo-library.yml": strings.ReplaceAll(libraryConfig, "{{title}}", titleFrom(filepath.Base(abs))),
		".gitignore":         "# Built output. \"octavo render\" recreates it.\npublic/\n_book/\n",
	}
	for name, content := range files {
		if _, err := os.Stat(filepath.Join(abs, name)); err == nil {
			continue
		}
		if err := fsutil.WriteFile(filepath.Join(abs, name), []byte(content)); err != nil {
			return err
		}
	}
	return nil
}
