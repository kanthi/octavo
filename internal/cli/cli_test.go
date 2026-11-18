package cli

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanthi/octavo/internal/format/pdf"
)

// run executes a octavo command and fails the test if it does not exit 0.
func run(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := Run(args, &out, &errOut); code != 0 {
		t.Fatalf("octavo %s: exit %d\n%s%s", strings.Join(args, " "), code, out.String(), errOut.String())
	}
	if errOut.Len() > 0 {
		t.Errorf("octavo %s: unexpected warnings:\n%s", strings.Join(args, " "), errOut.String())
	}
	return out.String()
}

func read(t *testing.T, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func contains(t *testing.T, name, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("%s: missing %q", name, want)
		}
	}
}

func appendConfig(t *testing.T, dir, yaml string) {
	t.Helper()
	file := filepath.Join(dir, "octavo.yml")
	if err := os.WriteFile(file, []byte(read(t, file)+yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Tests build only the website unless they ask for more: printing a PDF in
// every test would be slow, and needs a browser.
func TestMain(m *testing.M) {
	os.Setenv("OCTAVO_TO", "html")
	os.Exit(m.Run())
}

func TestNewAndRender(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-first-book")
	run(t, "new", dir)
	run(t, "render", "--dir", dir)
	out := filepath.Join(dir, "_book")

	for _, f := range []string{"assets/book.css", "assets/book.js", "assets/chroma.css", "custom.css", "images/cover.svg", "search.json"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing output file %s", f)
		}
	}

	index := read(t, out, "index.html")
	contains(t, "index.html", index,
		"<title>Preface – My First Book</title>",
		`href="assets/book.css"`, `href="custom.css"`,
		`<a href="chapters/01-intro.html">the introduction</a>`, // .md link rewritten
		`src="images/cover.svg"`,
		`class="next" href="chapters/01-intro.html"`,
	)
	if strings.Contains(index, "<h1>Preface</h1>\n<h1") || strings.Count(index, "<h1") != 1 {
		t.Error("index.html: the chapter title should appear as exactly one h1")
	}
	if strings.Contains(index, "EventSource") {
		t.Error("index.html: live reload must not be in a normal render")
	}

	intro := read(t, out, "chapters", "01-intro.html")
	contains(t, "01-intro.html", intro,
		`data-root="../"`, `href="../assets/book.css"`, `href="../custom.css"`,
		`<span class="number">1</span>Introduction</h1>`,
		`<div class="callout callout-note" role="note">`, `<p class="callout-title">Warning</p>`,
		`<a href="02-basics.html">the basics</a>`,
		`<a href="#callouts">Callouts</a>`, // on this page
		`class="footnotes"`,
		`class="prev" href="../index.html"`,
	)
	if strings.Contains(intro, "[!NOTE]") {
		t.Error("01-intro.html: callout marker was left in the text")
	}

	basics := read(t, out, "chapters", "02-basics.html")
	contains(t, "02-basics.html", basics,
		"<title>The basics – My First Book</title>", // title from front matter
		`<pre class="chroma" data-lang="`, "<table>", `type="checkbox"`,
	)
	contains(t, "search.json", read(t, out, "search.json"), `"url":"chapters/02-basics.html"`, "Hello, book")
	contains(t, "chroma.css", read(t, out, "assets", "chroma.css"), `[data-theme="dark"] .chroma`)
}

func TestNewRefusesExistingBook(t *testing.T) {
	dir := t.TempDir()
	run(t, "new", dir)
	var out, errOut bytes.Buffer
	if code := Run([]string{"new", dir}, &out, &errOut); code == 0 {
		t.Fatal("second \"new\" in the same folder should fail")
	}
}

func TestPublishFolderAndCommand(t *testing.T) {
	dir, site, marker := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "ran")
	run(t, "new", dir)
	appendConfig(t, dir, "\npublish:\n  folder:\n    path: "+site+"\n  cloudflare:\n    type: command\n    run: ls {dir}/index.html > "+marker+"\n")

	// Two targets: one must be named.
	var out, errOut bytes.Buffer
	if code := Run([]string{"publish", "--dir", dir}, &out, &errOut); code == 0 {
		t.Fatal("publish without a target should fail when two are configured")
	}

	run(t, "publish", "folder", "--dir", dir)
	contains(t, "published index.html", read(t, site, "index.html"), "Preface")

	run(t, "publish", "cloudflare", "--dir", dir)
	contains(t, "command output", read(t, marker), "index.html")
}

func TestPublishGitHubPages(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir, remote := t.TempDir(), filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	run(t, "new", dir)
	appendConfig(t, dir, "\npublish:\n  gh-pages:\n    remote: "+remote+"\n    cname: book.example.com\n")
	run(t, "publish", "--dir", dir) // the only target, so no name is needed

	files, err := exec.Command("git", "--git-dir", remote, "ls-tree", "-r", "--name-only", "gh-pages").CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-tree: %v\n%s", err, files)
	}
	contains(t, "gh-pages branch", string(files), ".nojekyll", "CNAME", "index.html", "chapters/01-intro.html", "assets/book.css")
}

func TestBadConfig(t *testing.T) {
	cases := map[string]string{
		"no title":         "book:\n  chapters: [index.md]\n",
		"no chapters":      "book:\n  title: T\n",
		"outside the book": "book:\n  title: T\n  chapters: [../x.md]\n",
		"unknown format":   "book:\n  title: T\n  chapters: [index.md]\nformat:\n  docx: true\n",
		"bad theme":        "book:\n  title: T\n  chapters: [index.md]\nformat:\n  html:\n    theme: pink\n",
	}
	for name, yaml := range cases {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "octavo.yml"), []byte(yaml), 0o644)
		os.WriteFile(filepath.Join(dir, "index.md"), []byte("# Hi\n"), 0o644)
		var out, errOut bytes.Buffer
		if code := Run([]string{"render", "--dir", dir}, &out, &errOut); code == 0 {
			t.Errorf("%s: render should fail", name)
		}
	}
}

func TestLibrary(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "my-library")
	run(t, "new", "--library", lib)
	run(t, "add", "--dir", lib, "Linux")
	run(t, "add", "--dir", lib, "Go")

	out := run(t, "render", "--dir", lib)
	contains(t, "render output", out, "Built 2 of 2 books")

	// Every book has its own build folder and a copy in the public folder.
	for _, f := range []string{
		"books/Linux/_book/index.html", "books/Go/_book/chapters/01-intro.html",
		"public/index.html", "public/html/Linux/index.html", "public/html/Go/chapters/01-intro.html", "public/html/Go/assets/book.css",
	} {
		if _, err := os.Stat(filepath.Join(lib, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	portal := read(t, lib, "public", "index.html")
	contains(t, "portal", portal, "<title>My Library</title>", `href="html/Linux/index.html"`, `href="html/Go/index.html"`, "3 chapters")
	// Book pages link back to the portal, from any depth.
	contains(t, "book page", read(t, lib, "public", "html", "Go", "index.html"), `class="home-link" href="../../index.html"`)
	contains(t, "nested book page", read(t, lib, "public", "html", "Go", "chapters", "01-intro.html"), `class="home-link" href="../../../index.html"`)

	// Building one book (any letter case) keeps the other in the portal.
	if err := os.WriteFile(filepath.Join(lib, "books", "Go", "index.md"), []byte("# Changed preface\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	contains(t, "render output", run(t, "render", "go", "--dir", lib), "Built 1 of 2 books")
	contains(t, "rebuilt book", read(t, lib, "public", "html", "Go", "index.html"), "Changed preface")
	contains(t, "portal after one book", read(t, lib, "public", "index.html"), `href="html/Linux/index.html"`, `href="html/Go/index.html"`)

	// A broken book fails the run but the others are still built.
	os.WriteFile(filepath.Join(lib, "books", "Go", "octavo.yml"), []byte("book: {title: Go, chapters: [missing.md]}\n"), 0o644)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"render", "--dir", lib}, &stdout, &stderr); code == 0 {
		t.Fatal("render should fail when a book is broken")
	}
	contains(t, "failed run", stdout.String()+stderr.String(), "FAILED", "1 of 2 books failed: Go")
	if _, err := os.Stat(filepath.Join(lib, "public", "html", "Linux", "index.html")); err != nil {
		t.Error("the working book should still be published")
	}
}

func TestPartsAndSections(t *testing.T) {
	dir := t.TempDir()
	run(t, "new", dir)
	os.WriteFile(filepath.Join(dir, "octavo.yml"), []byte(`book:
  title: Parts
  chapters:
    - index.md
    - part: Commands
      contents:
        - section: Files
          contents:
            - text: Short label
              file: chapters/01-intro.md
    - part: Other
      contents:
        - chapters/02-basics.md
`), 0o644)
	run(t, "render", "--dir", dir)
	intro := read(t, dir, "_book", "chapters", "01-intro.html")
	contains(t, "01-intro.html", intro,
		"<summary>Commands</summary>", "<details open><summary>Files</summary>", // open: holds this page
		`aria-current="page">Short label</a>`,            // no numbers in the list by default
		`<span class="number">1</span>Introduction</h1>`, // the page keeps its own title
		`class="next" href="../chapters/02-basics.html"`,
	)
	contains(t, "index.html", read(t, dir, "_book", "index.html"), "<details><summary>Files</summary>") // closed elsewhere
}

func exists(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

func TestFormats(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-first-book")
	run(t, "new", dir)
	out := filepath.Join(dir, "_book")

	// --to names the formats for one run, whatever octavo.yml enables.
	run(t, "render", "--dir", dir, "--to", "epub")
	if !exists(out, "my-first-book.epub") || exists(out, "index.html") {
		t.Fatal("--to epub should write only the EPUB")
	}

	// The EPUB is a zip whose first entry is an uncompressed "mimetype",
	// and every page in it is well-formed XML.
	z, err := zip.OpenReader(filepath.Join(out, "my-first-book.epub"))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if first := z.File[0]; first.Name != "mimetype" || first.Method != zip.Store {
		t.Errorf("first entry is %q (method %d), want an uncompressed mimetype", first.Name, first.Method)
	}
	files := map[string]string{}
	for _, f := range z.File {
		r, _ := f.Open()
		data, _ := io.ReadAll(r)
		r.Close()
		files[f.Name] = string(data)
		if ext := filepath.Ext(f.Name); ext == ".xhtml" || ext == ".opf" || ext == ".xml" {
			d := xml.NewDecoder(bytes.NewReader(data))
			for {
				if _, err := d.Token(); err == io.EOF {
					break
				} else if err != nil {
					t.Errorf("%s is not well-formed XML: %v", f.Name, err)
					break
				}
			}
		}
	}
	contains(t, "content.opf", files["OEBPS/content.opf"],
		"<dc:title>My First Book</dc:title>", `href="text/ch003.xhtml"`, `properties="nav"`, `media-type="image/svg+xml"`)
	contains(t, "nav.xhtml", files["OEBPS/nav.xhtml"], `<a href="text/ch002.xhtml">1. Introduction</a>`)
	contains(t, "ch001.xhtml", files["OEBPS/text/ch001.xhtml"],
		`<a href="ch002.xhtml">the introduction</a>`, // a chapter link points inside the EPUB
		`src="../images/images/cover.svg"`)
	if _, ok := files["OEBPS/images/images/cover.svg"]; !ok {
		t.Error("the image used in the preface is not in the EPUB")
	}

	// Built together, the website links to the book files.
	run(t, "render", "--dir", dir, "--to", "html,epub")
	contains(t, "index.html", read(t, out, "index.html"), `<a href="my-first-book.epub" download>EPUB</a>`)
	contains(t, "nested page", read(t, out, "chapters", "01-intro.html"), `<a href="../my-first-book.epub" download>EPUB</a>`)

	// Building one format keeps the files of the others.
	run(t, "render", "--dir", dir, "--to", "html")
	if !exists(out, "my-first-book.epub") || !exists(out, "index.html") {
		t.Error("--to html should add the website and keep the EPUB")
	}
	// Without --to (and without OCTAVO_TO), a render is a clean build of
	// every format in octavo.yml, and a new book lists all three. The PDF is
	// skipped with a warning on a machine with no browser, so warnings are
	// not checked here.
	t.Setenv("OCTAVO_TO", "")
	file := filepath.Join(dir, "octavo.yml")
	if err := os.WriteFile(file, []byte(strings.Replace(read(t, file), "epub: {}", "epub:\n    output-file: book", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	var plainOut, plainErr bytes.Buffer
	if code := Run([]string{"render", "--dir", dir}, &plainOut, &plainErr); code != 0 {
		t.Fatalf("plain render: exit %d\n%s%s", code, plainOut.String(), plainErr.String())
	}
	if !exists(out, "book.epub") || !exists(out, "index.html") || exists(out, "my-first-book.epub") {
		t.Error("a plain render should clear the folder and build the website and the EPUB")
	}
	if _, err := pdf.FindBrowser(""); err == nil && !exists(out, "my-first-book.pdf") {
		t.Error("a plain render should build the PDF when a browser is installed")
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"render", "--dir", dir, "--to", "docx"}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), `unknown format "docx"`) {
		t.Errorf("--to docx: exit %d, %q", code, stderr.String())
	}
}

// Both PDF engines, where installed. The Typst one also builds the guide,
// which uses every feature a chapter can have.
func TestPDF(t *testing.T) {
	for _, engine := range []string{"typst", "chrome"} {
		t.Run(engine, func(t *testing.T) {
			find := map[string]func(string) (string, error){"typst": pdf.FindTypst, "chrome": pdf.FindBrowser}[engine]
			if _, err := find(""); err != nil {
				t.Skipf("%s is not installed", engine)
			}
			t.Setenv("OCTAVO_PDF_ENGINE", engine)
			dir := filepath.Join(t.TempDir(), "my-first-book")
			run(t, "new", dir)
			text := run(t, "render", "--dir", dir, "--to", "pdf,html")
			contains(t, "output", text, "my-first-book.pdf")
			if data := read(t, dir, "_book", "my-first-book.pdf"); !strings.HasPrefix(data, "%PDF-") || len(data) < 10000 {
				t.Errorf("not a PDF (%d bytes)", len(data))
			}
			if engine == "typst" {
				guide := filepath.Join(t.TempDir(), "guide")
				run(t, "guide", "--copy", guide)
				run(t, "render", "--dir", guide, "--to", "pdf")
				if data := read(t, guide, "_book", "octavo-guide.pdf"); !strings.HasPrefix(data, "%PDF-") || len(data) < 50000 {
					t.Errorf("guide: not a PDF (%d bytes)", len(data))
				}
			}
		})
	}
}

// Asking for Typst by name never falls back to the browser.
func TestPDFTypstMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-first-book")
	run(t, "new", dir)
	t.Setenv("OCTAVO_PDF_ENGINE", "typst")
	t.Setenv("OCTAVO_TYPST", filepath.Join(dir, "no-such-typst"))
	contains(t, "--to pdf", fail(t, "render", "--dir", dir, "--to", "pdf"), "engine is typst", "not found")
	t.Setenv("OCTAVO_PDF_ENGINE", "word")
	contains(t, "bad engine", fail(t, "render", "--dir", dir, "--to", "pdf"), "must be auto, typst or chrome")
}

func TestLibraryFormats(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	run(t, "new", "--library", lib)
	run(t, "add", "--dir", lib, "Go")
	run(t, "render", "--dir", lib, "--to", "html,epub")
	if !exists(lib, "public", "epub", "Go.epub") || exists(lib, "public", "html", "Go", "go.epub") {
		t.Error("the EPUB should be at public/epub/Go.epub and not inside the website copy")
	}
	contains(t, "portal", read(t, lib, "public", "index.html"), `href="html/Go/index.html"`, `<a href="epub/Go.epub" download>EPUB</a>`)
	contains(t, "book page", read(t, lib, "public", "html", "Go", "chapters", "01-intro.html"), `<a href="../../../epub/Go.epub" download>EPUB</a>`)
}
