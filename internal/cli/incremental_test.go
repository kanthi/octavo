package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/format"
	"github.com/kanthi/octavo/internal/library"
)

func edit(t *testing.T, file string, change func(string) string) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(change(string(data))), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A kept Output and Cache rebuild only the files whose inputs changed.
func TestIncrementalBook(t *testing.T) {
	dir, outDir := t.TempDir(), t.TempDir()
	run(t, "new", dir)
	out, cache := format.NewOutput(outDir), book.NewCache()
	build := func() []string {
		t.Helper()
		b, err := book.LoadCached(dir, cache)
		if err != nil {
			t.Fatal(err)
		}
		if err := format.RenderAll(b, out, format.Options{Formats: previewFormats}); err != nil {
			t.Fatal(err)
		}
		return out.Changed()
	}
	expect := func(what string, got []string, want ...string) {
		t.Helper()
		if len(got) == 0 && len(want) == 0 {
			return
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: rebuilt %v, want %v", what, got, want)
		}
	}

	if first := build(); len(first) != 30 {
		t.Errorf("first build wrote %d files, want all 30: %v", len(first), first)
	}
	expect("no edits", build())

	// Text of one chapter: that page and the search index.
	edit(t, filepath.Join(dir, "chapters", "01-intro.md"), func(s string) string { return s + "\nA new closing line.\n" })
	expect("chapter text", build(), "chapters/01-intro.html", "search.json")
	contains(t, "01-intro.html", read(t, outDir, "chapters", "01-intro.html"), "A new closing line.")

	// A chapter's title is in every sidebar: every page.
	edit(t, filepath.Join(dir, "chapters", "01-intro.md"), func(s string) string {
		return strings.Replace(s, "# Introduction", "# A longer introduction", 1)
	})
	expect("chapter title", build(), "chapters/01-intro.html", "chapters/02-basics.html", "index.html", "search.json")
	contains(t, "index.html", read(t, outDir, "index.html"), "A longer introduction")

	// A stylesheet or an image: only that file.
	edit(t, filepath.Join(dir, "custom.css"), func(s string) string { return s + "\nh1 { color: red; }\n" })
	expect("stylesheet", build(), "custom.css")
	edit(t, filepath.Join(dir, "images", "cover.svg"), func(s string) string { return s + "<!-- edited -->\n" })
	expect("image", build(), "images/cover.svg")

	// A book setting shown on every page: every page, nothing else.
	edit(t, filepath.Join(dir, "octavo.yml"), func(s string) string { return strings.Replace(s, "author: Your Name", "author: Someone Else", 1) })
	expect("book setting", build(), "chapters/01-intro.html", "chapters/02-basics.html", "index.html")

	// A chapter removed from the list and set aside ("_" in front, so it is
	// not added back by its number): its page is deleted, the others redone.
	edit(t, filepath.Join(dir, "octavo.yml"), func(s string) string { return strings.Replace(s, "    - chapters/02-basics.md\n", "", 1) })
	os.Rename(filepath.Join(dir, "chapters", "02-basics.md"), filepath.Join(dir, "chapters", "_02-basics.md"))
	expect("chapter removed", build(), "chapters/01-intro.html", "index.html", "search.json")
	if _, err := os.Stat(filepath.Join(outDir, "chapters", "02-basics.html")); !os.IsNotExist(err) {
		t.Error("the removed chapter's page should be deleted")
	}
}

// In a library, an edit in one book leaves the other books alone.
func TestIncrementalLibrary(t *testing.T) {
	lib, public := filepath.Join(t.TempDir(), "lib"), t.TempDir()
	run(t, "new", "--library", lib)
	run(t, "add", "--dir", lib, "Linux")
	run(t, "add", "--dir", lib, "Go")
	state := library.NewState()
	build := func() []string {
		t.Helper()
		l, err := library.Load(lib)
		if err != nil {
			t.Fatal(err)
		}
		res, err := l.Build(library.BuildOptions{PublicDir: public, Direct: true, State: state, Formats: previewFormats})
		if err != nil {
			t.Fatal(err)
		}
		return res.Changed
	}
	if first := build(); len(first) != 60 { // 29 files per book, the portal and the 404 page
		t.Errorf("first build wrote %d files, want 60: %v", len(first), first)
	}
	if again := build(); len(again) != 0 {
		t.Errorf("no edits, but rebuilt %v", again)
	}
	edit(t, filepath.Join(lib, "books", "Go", "index.md"), func(s string) string { return s + "\nMore.\n" })
	if got, want := build(), []string{"html/Go/index.html", "html/Go/search.json"}; !reflect.DeepEqual(got, want) {
		t.Errorf("one chapter edited: rebuilt %v, want %v", got, want)
	}
	// The book's title is on the portal and on each of its own pages.
	edit(t, filepath.Join(lib, "books", "Go", "octavo.yml"), func(s string) string { return strings.Replace(s, `title: "Go"`, `title: "Golang"`, 1) })
	got := build()
	for _, f := range got {
		if strings.HasPrefix(f, "html/Linux/") {
			t.Errorf("a change in Go rebuilt %s", f)
		}
	}
	if len(got) != 4 || got[len(got)-1] != "index.html" {
		t.Errorf("book title edited: rebuilt %v, want Go's 3 pages and the portal", got)
	}
}
