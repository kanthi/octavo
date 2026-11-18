package cli

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kanthi/octavo/internal/format"
	"github.com/kanthi/octavo/internal/publish"
)

// TestGuide builds the guide as a reader gets it and checks that it is whole:
// no warnings, the version filled in, and no broken link anywhere.
func TestGuide(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "guide")
	contains(t, "guide --copy", run(t, "guide", "--copy", dir), "octavo preview")
	fail(t, "guide", "--copy", dir) // never overwrites

	run(t, "render", "--dir", dir) // run fails the test on any warning
	out := filepath.Join(dir, "_book")
	index := read(t, out, "index.html")
	contains(t, "guide index", index, "Octavo "+Version)
	if strings.Contains(index, "{{") {
		t.Error("guide: a {{placeholder}} was left in the text")
	}
	if n := checkLinks(t, out); n < 200 {
		t.Errorf("guide: only %d local links checked; is the guide complete?", n)
	}
}

var linkAttr = regexp.MustCompile(`(?:href|src)="([^"]+)"`)

// checkLinks fails the test for every local link in the site that points at
// a missing file or a missing #id, and returns how many links it checked.
func checkLinks(t *testing.T, site string) int {
	t.Helper()
	ids := map[string]map[string]bool{}
	idsOf := func(file string) map[string]bool {
		if m, ok := ids[file]; ok {
			return m
		}
		m := map[string]bool{}
		data, _ := os.ReadFile(file)
		for _, id := range regexp.MustCompile(`id="([^"]+)"`).FindAllStringSubmatch(string(data), -1) {
			m[id[1]] = true
		}
		ids[file] = m
		return m
	}
	n := 0
	filepath.Walk(site, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(p) != ".html" {
			return err
		}
		data, _ := os.ReadFile(p)
		for _, m := range linkAttr.FindAllStringSubmatch(string(data), -1) {
			link := m[1]
			if strings.Contains(link, "://") || strings.HasPrefix(link, "mailto:") || strings.HasPrefix(link, "data:") {
				continue
			}
			n++
			u, err := url.Parse(link)
			if err != nil {
				t.Errorf("%s: bad link %q", p, link)
				continue
			}
			target := p
			if u.Path != "" {
				target = filepath.Join(filepath.Dir(p), filepath.FromSlash(u.Path))
			}
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s: broken link %q", rel(site, p), link)
				continue
			}
			if u.Fragment != "" && filepath.Ext(target) == ".html" && !idsOf(target)[u.Fragment] {
				t.Errorf("%s: link %q: no id %q there", rel(site, p), link, u.Fragment)
			}
		}
		return nil
	})
	return n
}

func rel(base, p string) string {
	if r, err := filepath.Rel(base, p); err == nil {
		return r
	}
	return p
}

// TestGuideDocumentsEverything fails when a flag, format or publish target is
// added without a word about it in the guide's reference.
func TestGuideDocumentsEverything(t *testing.T) {
	commands := read(t, "..", "..", "guide", "content", "06-reference", "01-commands.md")
	flagLine := regexp.MustCompile(`(?m)^\s+-(\S+)`)
	for _, cmd := range []string{"new", "add", "render", "preview", "publish", "check", "guide"} {
		if !strings.Contains(commands, "## octavo "+cmd) {
			t.Errorf("reference: no section for \"octavo %s\"", cmd)
		}
		var out, errOut bytes.Buffer
		Run([]string{cmd, "-h"}, &out, &errOut)
		for _, m := range flagLine.FindAllStringSubmatch(errOut.String(), -1) {
			if !strings.Contains(commands, "`--"+m[1]) {
				t.Errorf("reference: octavo %s --%s is not documented", cmd, m[1])
			}
		}
	}
	settings := read(t, "..", "..", "guide", "content", "06-reference", "02-book-settings.md")
	for _, name := range format.Names() {
		if !strings.Contains(settings, "### "+name) {
			t.Errorf("book settings: format %q has no section", name)
		}
	}
	for _, kind := range publish.Kinds() {
		if _, err := os.Stat(filepath.Join("..", "..", "guide", "content", "05-publishing", "targets", kind+".md")); err != nil {
			t.Errorf("publishing: target %q has no page", kind)
		}
	}
}

func TestGuideSaveAs(t *testing.T) {
	out := t.TempDir()
	contains(t, "guide --to epub", run(t, "guide", "--to", "epub", "--out", out), "Saved")
	file := filepath.Join(out, "Octavo-Guide-"+Version+".epub")
	if info, err := os.Stat(file); err != nil || info.Size() < 10000 {
		t.Fatalf("guide --to epub: no proper file at %s", file)
	}
	contains(t, "second save", fail(t, "guide", "--to", "epub", "--out", out), "already exists")
	fail(t, "guide", "--to", "html") // the website is opened, not saved
}

func TestZenModeAndPageTOC(t *testing.T) {
	dir := t.TempDir()
	run(t, "new", dir)
	run(t, "render", "--dir", dir)
	intro := read(t, dir, "_book", "chapters", "01-intro.html")
	contains(t, "page", intro, `id="zen-toggle"`, `data-panel="sidebar"`, `data-panel="toc"`, `data-panel="zen"`, `class="toc"`, "d.dataset.zen",
		`class="toc-toggle" data-toggle="toc"`)
	contains(t, "book.js", read(t, dir, "_book", "assets", "book.js"), `e.key === "z"`)
	contains(t, "book.css", read(t, dir, "_book", "assets", "book.css"), `html[data-zen] .sidebar`, `html[data-toc="hidden"] .toc`,
		`html[data-zen] .layout { max-width: none; }`, `--zen-size: clamp(`)

	cfg := filepath.Join(dir, "octavo.yml")
	os.WriteFile(cfg, []byte(strings.Replace(read(t, cfg), "    theme: auto", "    theme: auto\n    page-toc: false", 1)), 0o644)
	run(t, "render", "--dir", dir)
	intro = read(t, dir, "_book", "chapters", "01-intro.html")
	if strings.Contains(intro, `class="toc"`) || strings.Contains(intro, "toc-toggle") || !strings.Contains(intro, `data-page-toc="off"`) {
		t.Error(`page-toc: false should remove "On this page" and mark the page`)
	}
}

// Only the part holding the page is open in the chapter list, and books with
// parts get "Expand all"; books without parts do not.
func TestSidebarParts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "guide")
	run(t, "guide", "--copy", dir)
	run(t, "render", "--dir", dir)
	page := read(t, dir, "_book", "content", "02-writing", "01-chapters.html")
	contains(t, "page", page, `<details open><summary>Writing`, `<details><summary>Getting started`, `<details><summary>Reference`, `id="nav-all"`)
	contains(t, "book.js", read(t, dir, "_book", "assets", "book.js"), `saved.nav = "open"`)

	flat := t.TempDir()
	run(t, "new", flat)
	run(t, "render", "--dir", flat)
	if strings.Contains(read(t, flat, "_book", "chapters", "01-intro.html"), `id="nav-all"`) {
		t.Error(`a book without parts should not get "Expand all"`)
	}
}

// The chapter list shows titles without numbers unless sidebar-numbers is
// on; the chapter heading keeps its number either way.
func TestSidebarNumbers(t *testing.T) {
	dir := t.TempDir()
	run(t, "new", dir)
	run(t, "render", "--dir", dir, "--to", "html")
	sidebar := func() string {
		page := read(t, dir, "_book", "chapters", "01-intro.html")
		start, end := strings.Index(page, `<nav class="sidebar"`), strings.Index(page, "</nav>")
		if start < 0 || end < start {
			t.Fatal("no chapter list on the page")
		}
		contains(t, "heading", page, `<h1><span class="number">1</span>`)
		return page[start:end]
	}
	if strings.Contains(sidebar(), `class="number"`) {
		t.Error("the chapter list should have no numbers by default")
	}
	cfg := filepath.Join(dir, "octavo.yml")
	os.WriteFile(cfg, []byte(strings.Replace(read(t, cfg), "    theme: auto", "    theme: auto\n    sidebar-numbers: true", 1)), 0o644)
	run(t, "render", "--dir", dir, "--to", "html")
	contains(t, "chapter list", sidebar(), `<span class="number">1</span>`)
}
