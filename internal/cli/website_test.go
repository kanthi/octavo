package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runWarn runs a command that must succeed and returns its warnings.
func runWarn(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := Run(args, &out, &errOut); code != 0 {
		t.Fatalf("octavo %s: exit %d\n%s%s", strings.Join(args, " "), code, out.String(), errOut.String())
	}
	return errOut.String()
}

func TestRedirectsForMovedChapters(t *testing.T) {
	dir := t.TempDir()
	run(t, "new", dir)
	run(t, "render", "--dir", dir, "--to", "html")
	contains(t, "octavo-pages.txt", read(t, dir, "octavo-pages.txt"),
		"chapters/01-intro.html\tIntroduction", "chapters/02-basics.html\tThe basics")

	// Move one chapter into another folder, and rename the other's file.
	cfg := filepath.Join(dir, "octavo.yml")
	os.MkdirAll(filepath.Join(dir, "moved"), 0o755)
	os.Rename(filepath.Join(dir, "chapters", "02-basics.md"), filepath.Join(dir, "moved", "02-basics.md"))
	os.Rename(filepath.Join(dir, "chapters", "01-intro.md"), filepath.Join(dir, "chapters", "welcome.md"))
	edit(t, cfg, func(s string) string {
		s = strings.Replace(s, "chapters/02-basics.md", "moved/02-basics.md", 1)
		return strings.Replace(s, "chapters/01-intro.md", "chapters/welcome.md", 1)
	})
	// Images and links in the moved chapter now point to the wrong place,
	// which is the author's to fix; only the warnings about that may appear.
	runWarn(t, "render", "--dir", dir, "--to", "html")

	basics := read(t, dir, "_book", "chapters", "02-basics.html")
	contains(t, "old basics page", basics, `url=../moved/02-basics.html`, `name="robots" content="noindex"`)
	intro := read(t, dir, "_book", "chapters", "01-intro.html") // same title, new file name
	contains(t, "old intro page", intro, `url=../chapters/welcome.html`)
	contains(t, "octavo-pages.txt", read(t, dir, "octavo-pages.txt"),
		"chapters/02-basics.html\t-> moved/02-basics.html", "moved/02-basics.html\tThe basics")
	if strings.Contains(read(t, dir, "_book", "search.json"), "02-basics.html\"") &&
		!strings.Contains(read(t, dir, "_book", "search.json"), "moved/02-basics.html") {
		t.Error("search should list the new page")
	}

	// A second move: the oldest address follows the chain to the newest.
	os.Rename(filepath.Join(dir, "moved", "02-basics.md"), filepath.Join(dir, "moved", "basics.md"))
	edit(t, cfg, func(s string) string { return strings.Replace(s, "moved/02-basics.md", "moved/basics.md", 1) })
	runWarn(t, "render", "--dir", dir, "--to", "html")
	contains(t, "oldest basics page", read(t, dir, "_book", "chapters", "02-basics.html"), `url=../moved/basics.html`)
	contains(t, "older basics page", read(t, dir, "_book", "moved", "02-basics.html"), `url=../moved/basics.html`)

	// A page that matches nothing is reported, once.
	edit(t, cfg, func(s string) string { return strings.Replace(s, "    - moved/basics.md\n", "", 1) })
	warnings := runWarn(t, "render", "--dir", dir, "--to", "html")
	contains(t, "warnings", warnings, "moved/basics.html (\"The basics\") is no longer in the book")
	if again := runWarn(t, "render", "--dir", dir, "--to", "html"); strings.Contains(again, "no longer in the book") {
		t.Errorf("the same page was reported twice:\n%s", again)
	}

	// Named redirects, to a chapter's section or another site.
	edit(t, cfg, func(s string) string {
		return strings.Replace(s, "  html:\n", "  html:\n    redirects:\n      old/faq.html: chapters/welcome.md#start\n      gone.md: https://example.com/elsewhere\n", 1)
	})
	runWarn(t, "render", "--dir", dir, "--to", "html")
	contains(t, "named redirect", read(t, dir, "_book", "old", "faq.html"), `url=../chapters/welcome.html#start`)
	contains(t, "redirect to a URL", read(t, dir, "_book", "gone.html"), `url=https://example.com/elsewhere`)
}

func TestSetupGitHub(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := t.TempDir()
	git := func(args ...string) {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "trunk")
	git("remote", "add", "origin", "git@github.com:Someone/notes.git")
	dir := filepath.Join(repo, "books", "my-book")
	run(t, "new", dir)

	out := run(t, "publish", "--setup", "github", "--dir", dir)
	contains(t, "message", out, "https://github.com/Someone/notes/settings/pages", "https://someone.github.io/notes/", "push to trunk")
	wf := read(t, repo, ".github", "workflows", "octavo.yml")
	contains(t, "workflow", wf,
		"branches: [trunk]",
		"octavo render --dir books/my-book",
		"path: books/my-book/_book",
		`export OCTAVO_SITE_URL="${BASE_URL/#http:/https:}/"`, "BASE_URL: ${{ steps.pages.outputs.base_url }}",
		"actions/deploy-pages@")
	if !strings.Contains(fail(t, "publish", "--setup", "github", "--dir", dir), "already exists") {
		t.Error("a second setup should refuse to overwrite the workflow")
	}
	if !strings.Contains(fail(t, "publish", "--setup", "gitlab", "--dir", dir), "unknown service") {
		t.Error("an unknown service should be refused")
	}
}

func TestSiteURLFromEnvironment(t *testing.T) {
	dir := t.TempDir()
	run(t, "new", dir)
	t.Setenv("OCTAVO_SITE_URL", "https://example.org/book/")
	run(t, "render", "--dir", dir, "--to", "html")
	contains(t, "sitemap", read(t, dir, "_book", "sitemap.xml"), "<loc>https://example.org/book/chapters/01-intro.html</loc>")
}

func TestVersions(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	run(t, "new", dir)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "first edition")
	git("tag", "v1")

	// The second edition renames a chapter and adds versions.
	edit(t, filepath.Join(dir, "chapters", "02-basics.md"), func(s string) string {
		return strings.Replace(s, "title: The basics", "title: The basics, revised", 1)
	})
	appendConfig(t, dir, `
versions:
  - label: 2nd edition
  - label: 1st edition
    ref: v1
`)
	out := run(t, "render", "--dir", dir, "--to", "html")
	contains(t, "render", out, "in 2 versions, the latest in 2nd-edition/")

	latest := read(t, dir, "_book", "2nd-edition", "chapters", "02-basics.html")
	contains(t, "latest", latest, "The basics, revised",
		`<option value="../../2nd-edition/" selected>2nd edition</option>`,
		`<option value="../../1st-edition/">1st edition</option>`)
	if strings.Contains(latest, "version-note") {
		t.Error("the latest version should have no note about older versions")
	}
	older := read(t, dir, "_book", "1st-edition", "chapters", "02-basics.html")
	contains(t, "older", older, "<h1><span class=\"number\">2</span>The basics</h1>",
		`<option value="../../1st-edition/" selected>`,
		`class="version-note"`, `href="../../2nd-edition/index.html" data-version-link>Go to 2nd edition</a>`)
	// Addresses without a version lead to the latest.
	contains(t, "root page", read(t, dir, "_book", "chapters", "02-basics.html"), `url=../2nd-edition/chapters/02-basics.html`)
	contains(t, "root index", read(t, dir, "_book", "index.html"), `url=2nd-edition/index.html`)
	if !exists(dir, "_book", "404.html") {
		t.Error("no 404 page at the root")
	}
	// The temporary checkout is gone, and the working folder untouched.
	if out, _ := exec.Command("git", "-C", dir, "worktree", "list").Output(); strings.Count(string(out), "\n") != 1 {
		t.Errorf("a git worktree was left behind:\n%s", out)
	}

	appendConfig(t, dir, "  - label: Missing\n    ref: no-such-tag\n")
	contains(t, "bad ref", fail(t, "render", "--dir", dir, "--to", "html"), `no branch, tag or commit "no-such-tag"`)
}

func TestOfflineReading(t *testing.T) {
	dir := t.TempDir()
	run(t, "new", dir)
	run(t, "render", "--dir", dir, "--to", "html")
	sw := read(t, dir, "_book", "sw.js")
	contains(t, "sw.js", sw, `var SHELL = ["assets/book.css","assets/chroma.css","assets/book.js"];`)
	if strings.Contains(sw, "{{") {
		t.Error("sw.js has a placeholder left")
	}
	page := read(t, dir, "_book", "chapters", "01-intro.html")
	contains(t, "page", page, `data-offline="on"`, `id="offline-save"`)

	// A new build with other styles gets a new cache name.
	edit(t, filepath.Join(dir, "octavo.yml"), func(s string) string {
		return strings.Replace(s, "    theme: auto", "    theme: auto\n    highlight-light: monokailight", 1)
	})
	run(t, "render", "--dir", dir, "--to", "html")
	if read(t, dir, "_book", "sw.js") == sw {
		t.Error("sw.js should change when the styles change")
	}

	edit(t, filepath.Join(dir, "octavo.yml"), func(s string) string {
		return strings.Replace(s, "    theme: auto", "    theme: auto\n    offline: false", 1)
	})
	run(t, "render", "--dir", dir, "--to", "html")
	if exists(dir, "_book", "sw.js") {
		t.Error("offline: false should leave out sw.js")
	}
	if p := read(t, dir, "_book", "chapters", "01-intro.html"); strings.Contains(p, `data-offline="on"`) {
		t.Error("offline: false should leave offline reading off the page")
	}
}

func TestAutoAddChapters(t *testing.T) {
	dir := t.TempDir()
	run(t, "new", dir)
	chapter := func(rel, title string) {
		t.Helper()
		write(t, dir, rel, "---\ntitle: "+title+"\n---\n\nText.\n")
	}
	// Numbered files beside the chapters, a numbered new folder, and files
	// that must stay out.
	chapter("chapters/01a-between.md", "Between")
	chapter("chapters/03-after.md", "After")
	chapter("chapters/00-first.md", "First")
	chapter("chapters/notes.md", "Notes")
	chapter("chapters/_draft.md", "Draft")
	chapter("chapters/README.md", "Readme")
	write(t, dir, "chapters/part.md", "{{< include _bit.md >}}\n")
	write(t, dir, "chapters/_bit.md", "Included.\n")

	warnings := runWarn(t, "render", "--dir", dir, "--to", "html")
	contains(t, "warnings", warnings, "chapters/notes.md: not added to the book: its name has no number", "chapters/part.md: not added")
	for _, quiet := range []string{"_draft", "README", "_bit"} {
		if strings.Contains(warnings, quiet) {
			t.Errorf("%s should be left alone quietly:\n%s", quiet, warnings)
		}
	}
	search := read(t, dir, "_book", "search.json")
	order := []string{"chapters/00-first.html", "chapters/01-intro.html", "chapters/01a-between.html", "chapters/02-basics.html", "chapters/03-after.html"}
	last := -1
	for _, page := range order {
		at := strings.Index(search, `"url":"`+page+`"`)
		if at < 0 {
			t.Fatalf("%s is not in the book:\n%s", page, search)
		}
		if at < last {
			t.Errorf("%s is out of order", page)
		}
		last = at
	}
	if strings.Contains(read(t, dir, "octavo.yml"), "01a-between") {
		t.Error("octavo.yml should not be changed")
	}

	// auto-add: false turns it off.
	edit(t, filepath.Join(dir, "octavo.yml"), func(s string) string {
		return strings.Replace(s, "  chapters:\n", "  auto-add: false\n  chapters:\n", 1)
	})
	runWarn(t, "render", "--dir", dir, "--to", "html")
	if strings.Contains(read(t, dir, "_book", "search.json"), "01a-between") {
		t.Error("auto-add: false should add nothing")
	}
}

func TestAutoAddFolders(t *testing.T) {
	dir := t.TempDir()
	run(t, "new", dir)
	// A book whose parts are folders, like the imported ones.
	for _, f := range []string{"content/01-start/01-a.md", "content/01-start/02-b.md", "content/03-end/01-z.md"} {
		write(t, dir, f, "# "+filepath.Base(f)+"\n")
	}
	edit(t, filepath.Join(dir, "octavo.yml"), func(s string) string {
		i := strings.Index(s, "  chapters:\n")
		j := strings.Index(s, "\nformat:")
		return s[:i] + `  chapters:
    - index.md
    - part: Start
      contents:
        - content/01-start/01-a.md
        - content/01-start/02-b.md
    - part: End
      contents:
        - content/03-end/01-z.md
` + s[j:]
	})
	// A new numbered folder between them, with an index and a subfolder.
	write(t, dir, "content/02-middle-of-the-road/index.md", "---\ntitle: The Middle\n---\n")
	write(t, dir, "content/02-middle-of-the-road/01-m.md", "# M\n")
	write(t, dir, "content/02-middle-of-the-road/05-deeper/01-d.md", "# D\n")
	write(t, dir, "content/04-no-index/01-n.md", "# N\n")

	out := run(t, "render", "--dir", dir, "--to", "html")
	contains(t, "render", out, "4 chapters not in octavo.yml, added by number")
	nav := read(t, dir, "_book", "content", "01-start", "01-a.html")
	nav = nav[strings.Index(nav, `<nav class="sidebar"`):strings.Index(nav, "</nav>")]
	start, middle, deeper, end, noIndex := strings.Index(nav, "Start"), strings.Index(nav, "The Middle"), strings.Index(nav, "<summary>Deeper</summary>"), strings.Index(nav, "<summary>End"), strings.Index(nav, "No Index")
	if !(start >= 0 && start < middle && middle < deeper && deeper < end && end < noIndex) {
		t.Errorf("parts out of order (start %d, middle %d, deeper %d, end %d, no index %d):\n%s", start, middle, deeper, end, noIndex, nav)
	}
}
