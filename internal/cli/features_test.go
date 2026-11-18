package cli

import (
	"archive/zip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`

// writingBook is a small book that uses every writing feature.
func writingBook(t *testing.T) string {
	dir := t.TempDir()
	write(t, dir, "octavo.yml", `book:
  title: Features
  author: Ann
  site-url: https://example.com/features
  chapters:
    - index.md
    - one.md
variables:
  version: "2.1"
  tool:
    name: Octavo
format:
  html: {}
  epub: {}
`)
	write(t, dir, "index.md", "# Preface\n\nHello.\n")
	write(t, dir, "one.md", `# One

Version {{< var version >}} of {{< var tool.name >}}, in {{< meta title >}}. Written as is: {{{< var version >}}}.

"Quoted" -- and --- done...

{{< include parts/shared.md >}}

`+"```markdown\n{{< var not-expanded >}}\n{{< include nothing.md >}}\n```"+`

![A diagram](img/d.svg "The whole system"){width=60%}

![Plain alt only](img/d.svg)

Inline ![icon](img/d.svg) in text is not a figure.

![](img/d.svg)

::: {.content-visible when-format="html"}
## Web only

Only on the website.
:::

::: {.content-hidden when-format="html"}
## Not on the web

In the e-book.
:::

## Last
`)
	write(t, dir, "parts/shared.md", "---\ntitle: ignored\n---\nShared text from an include.\n")
	write(t, dir, "img/d.svg", svg)
	write(t, dir, "img/d-dark.svg", svg)
	return dir
}

func TestWritingFeatures(t *testing.T) {
	dir := writingBook(t)
	run(t, "render", "--dir", dir, "--to", "html,epub")
	page := read(t, dir, "_book", "one.html")
	contains(t, "page", page,
		"Version 2.1 of Octavo, in Features.", "Written as is: {{&lt; var version &gt;}}.",
		"&ldquo;Quoted&rdquo; &ndash; and &mdash; done&hellip;",
		"Shared text from an include.",
		"{{&lt; var not-expanded &gt;}}", // code blocks are left alone
		`<figure id="figure-1">`, `<span class="figure-number">Figure 1.1</span> The whole system</figcaption>`,
		`<span class="figure-number">Figure 1.2</span> Plain alt only`,
		`style="width: 60%"`,
		`class="light-only"`, `src="img/d-dark.svg" alt="A diagram" title="The whole system" class="dark-only"`,
		`loading="lazy"`,
		"Only on the website.", `<a href="#web-only">Web only</a>`, // and in "On this page"
		`class="heading-link" href="#last"`,
		"Last updated <time",
	)
	if strings.Contains(page, "ignored") || strings.Contains(page, "In the e-book.") || strings.Contains(page, "not-on-the-web\">") {
		t.Error("included front matter, or content for other formats, reached the website")
	}
	if strings.Count(page, "<figure") != 2 {
		t.Errorf("want 2 figures (inline and alt-less images are not figures), got %d", strings.Count(page, "<figure"))
	}
	if !exists(dir, "_book", "img", "d-dark.svg") {
		t.Error("the dark version of an image was not copied")
	}

	// The EPUB has the other format block, and figures, but no dark images.
	epub := epubFile(t, filepath.Join(dir, "_book", "features.epub"), "ch002.xhtml")
	contains(t, "epub chapter", epub, "In the e-book.", "Figure 1.1")
	if strings.Contains(epub, "Only on the website.") || strings.Contains(epub, "dark-only\"") || strings.Contains(epub, "loading=") {
		t.Error("epub: got website-only content")
	}

	// Site files.
	contains(t, "sitemap", read(t, dir, "_book", "sitemap.xml"), "<loc>https://example.com/features/one.html</loc>")
	contains(t, "404", read(t, dir, "_book", "404.html"), "Page not found", `href="https://example.com/features/"`)
	contains(t, "head", page, `<link rel="icon" href="assets/favicon.svg">`, `<meta property="og:url" content="https://example.com/features/one.html">`, `<link rel="canonical"`)
	if !exists(dir, "_book", "assets", "favicon.svg") {
		t.Error("no default favicon")
	}
	contains(t, "search", read(t, dir, "_book", "search.json"), "Only on the website", "“Quoted”")
}

func epubFile(t *testing.T, file, suffix string) string {
	t.Helper()
	z, err := zip.OpenReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, f := range z.File {
		if strings.HasSuffix(f.Name, suffix) {
			r, _ := f.Open()
			data, _ := io.ReadAll(r)
			r.Close()
			return string(data)
		}
	}
	t.Fatalf("%s: no %s inside", file, suffix)
	return ""
}

func TestWritingErrors(t *testing.T) {
	cases := map[string]string{
		"unknown variable":  "{{< var nope >}}\n",
		"missing include":   "{{< include gone.md >}}\n",
		"include outside":   "{{< include ../x.md >}}\n",
		"include in a line": "text {{< include a.md >}} text\n",
		"unclosed block":    "::: {.content-visible when-format=\"pdf\"}\ntext\n",
		"unknown format":    "::: {.content-visible when-format=\"docx\"}\ntext\n:::\n",
		"include loop":      "{{< include index.md >}}\n",
	}
	for name, body := range cases {
		dir := t.TempDir()
		write(t, dir, "octavo.yml", "book:\n  title: T\n  chapters: [index.md]\n")
		write(t, dir, "index.md", "# T\n\n"+body)
		write(t, dir, "a.md", "A\n")
		out := fail(t, "render", "--dir", dir)
		if !strings.Contains(out, "index.md") {
			t.Errorf("%s: the error should name the file: %s", name, out)
		}
	}
	// Without a site address there is no sitemap, and the 404 page links to "/".
	dir := t.TempDir()
	run(t, "new", dir)
	run(t, "render", "--dir", dir)
	if exists(dir, "_book", "sitemap.xml") {
		t.Error("sitemap.xml without site-url")
	}
	contains(t, "404", read(t, dir, "_book", "404.html"), `href="/"`)
}

func TestIncludeChangeRebuilds(t *testing.T) {
	dir := writingBook(t)
	run(t, "render", "--dir", dir, "--to", "html")
	time.Sleep(10 * time.Millisecond)
	write(t, dir, "parts/shared.md", "Changed include.\n")
	run(t, "render", "--dir", dir, "--to", "html")
	contains(t, "after include edit", read(t, dir, "_book", "one.html"), "Changed include.")
}

func TestDatesFromGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	run(t, "new", dir)
	gitRun := func(date string, args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=T", "-c", "user.email=t@example.com"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitRun("2020-01-02T10:00:00", "init", "-q")
	gitRun("2020-01-02T10:00:00", "add", ".")
	gitRun("2020-01-02T10:00:00", "commit", "-q", "-m", "start")
	edit(t, filepath.Join(dir, "chapters", "02-basics.md"), func(s string) string { return s + "\nMore.\n" })
	gitRun("2021-03-04T10:00:00", "commit", "-q", "-am", "more")

	run(t, "render", "--dir", dir)
	contains(t, "intro", read(t, dir, "_book", "chapters", "01-intro.html"), `Last updated <time datetime="2020-01-02">`)
	contains(t, "basics", read(t, dir, "_book", "chapters", "02-basics.html"), `Last updated <time datetime="2021-03-04">`)
	contains(t, "first page", read(t, dir, "_book", "index.html"), `Started <time datetime="2020-01-02">`, `Updated <time datetime="2021-03-04">`)

	// An uncommitted change counts from the day it was saved: today.
	edit(t, filepath.Join(dir, "chapters", "01-intro.md"), func(s string) string { return s + "\nNew.\n" })
	run(t, "render", "--dir", dir)
	contains(t, "edited intro", read(t, dir, "_book", "chapters", "01-intro.html"), `Last updated <time datetime="`+time.Now().Format("2006-01-02")+`">`)

	// last-updated: false hides the dates.
	edit(t, filepath.Join(dir, "octavo.yml"), func(s string) string {
		return strings.Replace(s, "    theme: auto", "    theme: auto\n    last-updated: false", 1)
	})
	run(t, "render", "--dir", dir)
	if strings.Contains(read(t, dir, "_book", "chapters", "01-intro.html"), "Last updated") {
		t.Error("last-updated: false should hide the date")
	}
}

func TestLibrarySiteFiles(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	run(t, "new", "--library", lib, "--books", "Go")
	edit(t, filepath.Join(lib, "octavo-library.yml"), func(s string) string {
		return strings.Replace(s, "library:\n", "library:\n  site-url: https://books.example.com\n", 1)
	})
	run(t, "render", "--dir", lib)
	contains(t, "sitemap", read(t, lib, "public", "sitemap.xml"), "<loc>https://books.example.com/index.html</loc>", "<loc>https://books.example.com/html/Go/chapters/01-intro.html</loc>")
	contains(t, "404", read(t, lib, "public", "404.html"), `href="https://books.example.com"`)
	contains(t, "portal", read(t, lib, "public", "index.html"), "· Updated <time")
	contains(t, "book page", read(t, lib, "public", "html", "Go", "index.html"), `content="https://books.example.com/html/Go/index.html"`)
	if exists(lib, "public", "html", "Go", "404.html") || exists(lib, "public", "html", "Go", "sitemap.xml") {
		t.Error("a library book should leave the 404 page and sitemap to the library")
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "octavo.yml", "book:\n  title: T\n  chapters: [index.md, two.md]\n")
	write(t, dir, "index.md", `# T

[good](two.md#part-b) [bad anchor](two.md#nope) [here](#missing) [gone](gone.md) [unlisted](extra.md)
[web](https://example.com) [file](files/data.csv) [no file](files/none.csv)

![ok](img/a.svg) ![](img/a.svg) ![lost](img/lost.png)
`)
	write(t, dir, "two.md", "# Two\n\n## Part B\n\n## Go 1.27\n\n[dotted](#go-1.27)\n")
	write(t, dir, "extra.md", "Not listed.\n")
	write(t, dir, "notes/orphan.md", "Nobody links here.\n")
	write(t, dir, "README.md", "Ignored.\n")
	write(t, dir, "files/data.csv", "a,b\n")
	write(t, dir, "img/a.svg", svg)
	write(t, dir, "img/a-dark.svg", svg)
	write(t, dir, "img/unused.png", "png")
	write(t, dir, "img/cover.png", "png")
	write(t, dir, "style.css", "body { background: url(img/cover.png); }\n")

	out := fail(t, "check", "--dir", dir)
	contains(t, "check", out,
		`two.md#nope": two.md has no heading or figure with id "nope"`,
		`link "#missing": no heading or figure with that id in this chapter`,
		`link "gone.md": no such chapter`,
		`link "extra.md": that file is not in the chapter list`,
		`link "files/none.csv": no such file`,
		`image "img/lost.png": no such file`,
		`image "img/a.svg" has no description`,
		`(did you mean #go-127?)`,
		"notes/orphan.md: not in the chapter list",
		"img/unused.png: no chapter, stylesheet or setting uses this image",
		"7 errors, 3 warnings.",
	)
	for _, wrong := range []string{"README.md", "img/a-dark.svg", "img/cover.png", "#part-b", "data.csv\": no such"} {
		if strings.Contains(out, wrong) {
			t.Errorf("check should not report %q:\n%s", wrong, out)
		}
	}
	quiet := fail(t, "check", "--dir", dir, "--quiet")
	if strings.Contains(quiet, "warning ") {
		t.Error("--quiet should hide warnings")
	}

	clean := t.TempDir()
	run(t, "new", clean)
	contains(t, "clean book", run(t, "check", "--dir", clean), "no problems found", "0 errors, 0 warnings.")
}
