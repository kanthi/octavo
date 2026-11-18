package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fail runs a octavo command that must fail, and returns what it printed.
func fail(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := Run(args, &out, &errOut); code == 0 {
		t.Fatalf("octavo %s: should have failed\n%s", strings.Join(args, " "), out.String())
	}
	return out.String() + errOut.String()
}

func TestNewLibraryWithBooks(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	run(t, "new", "--library", lib, "--books", "Linux,Go")
	for _, name := range []string{"Linux", "Go"} {
		cfg := read(t, lib, "books", name, "octavo.yml")
		contains(t, name+" octavo.yml", cfg, "title: \""+name+"\"", "octavo-library.yml")
		if strings.Contains(cfg, "gh-pages") {
			t.Errorf("%s: a library book should not get its own publish section", name)
		}
		if exists(lib, "books", name, ".gitignore") {
			t.Errorf("%s: a library book should not get its own .gitignore", name)
		}
	}
	contains(t, "render", run(t, "render", "--dir", lib), "Built 2 of 2 books")

	// A bad name stops the whole command before anything is created.
	other := filepath.Join(t.TempDir(), "other")
	fail(t, "new", "--library", other, "--books", "Ok,../escape")
	if exists(other) {
		t.Error("a refused \"new --library\" should create nothing")
	}
}

func TestNewOnlyForStartingOut(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	run(t, "new", "--library", lib)
	contains(t, "new inside a library", fail(t, "new", filepath.Join(lib, "books", "Linux")), "octavo add")
	contains(t, "new library inside a library", fail(t, "new", "--library", filepath.Join(lib, "sub")), "octavo add")
	if exists(lib, "books", "Linux") || exists(lib, "sub") {
		t.Error("a refused \"new\" should create nothing")
	}
	fail(t, "new", "--books", "A", filepath.Join(t.TempDir(), "x")) // --books needs --library
}

func TestAddNeverTouchesExistingBooks(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	run(t, "new", "--library", lib)
	fail(t, "render", "--dir", lib) // no books yet

	contains(t, "add", run(t, "add", "--dir", lib, "Linux", "Go"), "Added book books/Linux", "Added book books/Go")
	mine := filepath.Join(lib, "books", "Linux", "index.md")
	if err := os.WriteFile(mine, []byte("# My own preface\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An existing name, in any letter case, is refused, and so is the whole
	// command: Rust is not created either.
	contains(t, "add existing", fail(t, "add", "--dir", lib, "Rust", "linux"), "already has a book \"Linux\"")
	if exists(lib, "books", "Rust") {
		t.Error("a refused \"add\" should create none of its books")
	}
	// A folder that is not a book is not touched either.
	os.MkdirAll(filepath.Join(lib, "books", "notes"), 0o755)
	contains(t, "add over a folder", fail(t, "add", "--dir", lib, "notes"), "already exists")
	fail(t, "add", "--dir", lib, "Go", "Go2", "go2") // named twice
	fail(t, "add", "--dir", lib, "a/b")
	fail(t, "add", "--dir", lib, ".hidden")
	fail(t, "add", "--dir", lib) // no name

	if got := read(t, mine); got != "# My own preface\n" {
		t.Errorf("an existing book was changed: %q", got)
	}
	contains(t, "render", run(t, "render", "--dir", lib), "Built 2 of 2 books")
}

func TestAddFromExistingBook(t *testing.T) {
	root := t.TempDir()
	single := filepath.Join(root, "my-book")
	run(t, "new", single)
	appendConfig(t, single, "\npublish:\n  folder:\n    path: /tmp/nowhere\n")
	run(t, "render", "--dir", single)
	os.MkdirAll(filepath.Join(single, ".git"), 0o755)
	os.WriteFile(filepath.Join(single, ".git", "HEAD"), []byte("ref"), 0o644)
	before := snapshot(t, single)

	// "add" in a single book explains how to make a library from it.
	contains(t, "add in a book", fail(t, "add", "--dir", single, "Other"), "octavo new --library", "--from")

	lib := filepath.Join(root, "lib")
	run(t, "new", "--library", lib)
	out := run(t, "add", "--dir", lib, "--from", single)
	contains(t, "add --from", out, "into books/my-book", "not changed", `"publish:" settings are not used`)
	if !exists(lib, "books", "my-book", "chapters", "01-intro.md") {
		t.Error("chapters were not copied")
	}
	for _, left := range []string{"_book", ".git"} {
		if exists(lib, "books", "my-book", left) {
			t.Errorf("%s should not be copied", left)
		}
	}
	// A second copy under its own name; the same name again is refused.
	run(t, "add", "--dir", lib, "--from", single, "Second")
	fail(t, "add", "--dir", lib, "--from", single)
	fail(t, "add", "--dir", lib, "--from", single, "A", "B")
	fail(t, "add", "--dir", lib, "--from", filepath.Join(root, "missing"))

	// A library inside the book it copies would copy itself.
	inner := filepath.Join(single, "lib")
	run(t, "new", "--library", inner)
	contains(t, "copy into itself", fail(t, "add", "--dir", inner, "--from", single), "inside the book being copied")
	os.RemoveAll(inner)

	if after := snapshot(t, single); after != before {
		t.Errorf("the original book was changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	contains(t, "render", run(t, "render", "--dir", lib), "Built 2 of 2 books")
}

// snapshot lists every file under dir with its contents, to compare.
func snapshot(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if info.IsDir() {
			b.WriteString(rel + "/\n")
			return nil
		}
		data, _ := os.ReadFile(p)
		b.WriteString(rel + " " + string(data) + "\n")
		return nil
	})
	return b.String()
}

func TestDrafts(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	run(t, "new", "--library", lib, "--books", "Linux,Rust")
	rust := filepath.Join(lib, "books", "Rust", "octavo.yml")
	os.WriteFile(rust, []byte(strings.Replace(read(t, rust), "book:\n", "book:\n  draft: true\n", 1)), 0o644)

	// All books: the draft is skipped and kept off the portal.
	out := run(t, "render", "--dir", lib)
	contains(t, "render", out, "Rust", "skipped: draft", "Built 1 of 2 books, skipped 1 draft (Rust)")
	if exists(lib, "public", "html", "Rust") {
		t.Error("a draft was built without --drafts")
	}
	if strings.Contains(read(t, lib, "public", "index.html"), "html/Rust/") {
		t.Error("a draft is on the portal without --drafts")
	}

	// Naming a draft is not enough; it needs --drafts.
	contains(t, "named draft", fail(t, "render", "--dir", lib, "rust"), "Rust is a draft", "--drafts")
	run(t, "render", "--dir", lib, "Rust", "--drafts")
	contains(t, "portal with drafts", read(t, lib, "public", "index.html"), `href="html/Rust/index.html"`, ">Draft<")

	// With --drafts every book is built, the draft with a badge.
	contains(t, "render --drafts", run(t, "render", "--dir", lib, "--drafts"), "Built 2 of 2 books.")
	contains(t, "portal", read(t, lib, "public", "index.html"), `href="html/Rust/index.html"`, `href="html/Linux/index.html"`)

	// Any later build without --drafts, even of one other book, removes the
	// draft's copy so that "publish --no-render" cannot send it.
	run(t, "render", "--dir", lib, "Linux")
	if exists(lib, "public", "html", "Rust") || strings.Contains(read(t, lib, "public", "index.html"), "html/Rust/") {
		t.Error("a draft built earlier with --drafts stayed in the public folder")
	}

	// Publishing leaves drafts out too.
	site := t.TempDir()
	os.WriteFile(filepath.Join(lib, "octavo-library.yml"), []byte(read(t, lib, "octavo-library.yml")+"publish:\n  folder:\n    path: "+site+"\n"), 0o644)
	run(t, "publish", "--dir", lib)
	if exists(site, "html", "Rust") || !exists(site, "html", "Linux", "index.html") {
		t.Error("publish should send Linux and leave the draft out")
	}
	run(t, "publish", "--dir", lib, "--drafts")
	if !exists(site, "html", "Rust", "index.html") {
		t.Error("publish --drafts should send the draft")
	}

	// Every book a draft: nothing to build.
	linux := filepath.Join(lib, "books", "Linux", "octavo.yml")
	os.WriteFile(linux, []byte(strings.Replace(read(t, linux), "book:\n", "book:\n  draft: true\n", 1)), 0o644)
	contains(t, "all drafts", fail(t, "render", "--dir", lib), "every book is a draft")

	// A single book marked as a draft needs --drafts as well.
	single := filepath.Join(t.TempDir(), "single")
	run(t, "new", single)
	cfg := filepath.Join(single, "octavo.yml")
	os.WriteFile(cfg, []byte(strings.Replace(read(t, cfg), "book:\n", "book:\n  draft: true\n", 1)), 0o644)
	contains(t, "single draft", fail(t, "render", "--dir", single), "draft", "--drafts")
	run(t, "render", "--dir", single, "--drafts")
}
