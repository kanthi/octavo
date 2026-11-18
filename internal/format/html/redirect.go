package html

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/format"
	"github.com/kanthi/octavo/internal/fsutil"
)

// Moved chapters keep their old addresses working. Octavo remembers every
// page address it has built in PagesFile, beside octavo.yml. When a page from
// that list is no longer built, its old address gets a small page that
// forwards the reader to where the chapter went:
//
//  1. the address given under "redirects:" in the html settings, if any;
//  2. else the chapter with the same title, if exactly one has it;
//  3. else the chapter with the same file name, if exactly one has it.
//
// A page that matches nothing is reported once and forgotten. The file is
// plain text so that its changes read well in git; commit it with the book,
// so that a build on another machine (or in CI) knows the old addresses too.

// PagesFile is the list of the website's page addresses, in the book folder.
const PagesFile = "octavo-pages.txt"

const pagesHeader = `# Every page address this book's website has had, kept by Octavo so that
# links to moved chapters keep working. Commit this file with the book.
# Each line: the page, a tab, then its chapter title, or "-> " and the page
# its old address now forwards to.
`

// history is the content of PagesFile: page -> title, or "-> page".
type history map[string]string

func readHistory(bookDir string) history {
	h := history{}
	data, err := os.ReadFile(filepath.Join(bookDir, PagesFile))
	if err != nil {
		return h
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		page, value, _ := strings.Cut(line, "\t")
		if page != "" {
			h[page] = value
		}
	}
	return h
}

func (h history) bytes() []byte {
	pages := make([]string, 0, len(h))
	for p := range h {
		pages = append(pages, p)
	}
	sort.Strings(pages)
	var b bytes.Buffer
	b.WriteString(pagesHeader)
	for _, p := range pages {
		fmt.Fprintf(&b, "%s\t%s\n", p, h[p])
	}
	return b.Bytes()
}

// redirects works out the forwarding pages and writes them. It returns the
// history to save: every current page plus every forwarded one.
func (f *Format) redirects(b *book.Book, out *format.Output, warn func(string, ...any)) (history, error) {
	current := map[string]*book.Chapter{}
	for _, ch := range b.Chapters {
		current[outPath(ch.Path)] = ch
	}
	byTitle, byName := map[string][]string{}, map[string][]string{}
	for page, ch := range current {
		byTitle[ch.Title] = append(byTitle[ch.Title], page)
		byName[path.Base(page)] = append(byName[path.Base(page)], page)
	}

	old := readHistory(b.Dir)
	next := history{}
	for page, ch := range current {
		next[page] = ch.Title
	}
	targets := map[string]string{} // old page -> address relative to the site root, or a full URL

	// Redirects named in octavo.yml come first.
	for from, to := range f.s.Redirects {
		from = strings.TrimPrefix(path.Clean(filepath.ToSlash(from)), "/")
		if strings.HasSuffix(from, ".md") {
			from = outPath(from)
		}
		if !strings.HasSuffix(from, ".html") || !filepath.IsLocal(filepath.FromSlash(from)) {
			warn("redirects: %q must be a page address inside the book, such as old/page.html", from)
			continue
		}
		if _, ok := current[from]; ok {
			warn("redirects: %s is still a page of the book, so it is not redirected", from)
			continue
		}
		target, ok := redirectTarget(to, current)
		if !ok {
			warn("redirects: %s -> %s: no such chapter in the book", from, to)
			continue
		}
		targets[from] = target
	}

	// Pages built before and gone now.
	for page, value := range old {
		if _, ok := current[page]; ok || page == "index.html" {
			continue
		}
		if _, ok := targets[page]; ok {
			continue
		}
		if to, ok := strings.CutPrefix(value, "-> "); ok {
			targets[page] = to
			continue
		}
		switch {
		case len(byTitle[value]) == 1:
			targets[page] = byTitle[value][0]
		case len(byName[path.Base(page)]) == 1:
			targets[page] = byName[path.Base(page)][0]
		default:
			warn("%s (%q) is no longer in the book, and Octavo cannot tell where it went; links to it will break. To forward it, add under format: html: redirects:\n    %s: chapters/new-place.md", page, value, page)
		}
	}

	// A forward may point at a page that has itself moved since; follow the
	// chain to a page that exists.
	pages := make([]string, 0, len(targets))
	for p := range targets {
		pages = append(pages, p)
	}
	sort.Strings(pages)
	for _, from := range pages {
		to := targets[from]
		for hops := 0; hops < 20; hops++ {
			page, _, _ := strings.Cut(to, "#")
			if _, ok := current[page]; ok || isURL(to) {
				break
			}
			further, ok := targets[page]
			if !ok {
				break
			}
			to = further
		}
		page, _, _ := strings.Cut(to, "#")
		if _, ok := current[page]; !ok && !isURL(to) {
			warn("redirect %s -> %s: that page is not in the book either", from, to)
			continue
		}
		next[from] = "-> " + to
		href := to
		if !isURL(to) {
			href = strings.Repeat("../", strings.Count(from, "/")) + to
		}
		if err := out.WriteData(from, RedirectPage(b.Config.Book.Title, href)); err != nil {
			return nil, err
		}
	}
	return next, nil
}

// redirectTarget turns the value of a redirects: entry (a chapter file, maybe
// with #section, or a full URL) into an address relative to the site root.
func redirectTarget(to string, current map[string]*book.Chapter) (string, bool) {
	if isURL(to) {
		return to, true
	}
	file, frag, _ := strings.Cut(to, "#")
	file = strings.TrimPrefix(path.Clean(filepath.ToSlash(file)), "/")
	if strings.HasSuffix(file, ".md") {
		file = outPath(file)
	}
	if _, ok := current[file]; !ok {
		return "", false
	}
	if frag != "" {
		file += "#" + frag
	}
	return file, true
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// redirectMarker is in every forwarding page, so a site-wide sitemap can
// leave them out (see IsRedirect).
const redirectMarker = `<meta name="octavo-redirect" content="moved">`

// RedirectPage forwards to href, keeping any #section of the old address
// when the new one names none.
func RedirectPage(title, href string) []byte {
	h := template.HTMLEscapeString(href)
	js := template.JSEscapeString(href)
	return []byte(fmt.Sprintf(`<!doctype html>
<html lang="en">
<meta charset="utf-8">
<title>Moved – %[1]s</title>
%[4]s
<meta name="robots" content="noindex">
<link rel="canonical" href="%[2]s">
<meta http-equiv="refresh" content="0; url=%[2]s">
<script>location.replace("%[3]s" + ("%[3]s".indexOf("#") < 0 ? location.hash : ""));</script>
<p>This page has moved: <a href="%[2]s">go to its new place</a>.</p>
</html>
`, template.HTMLEscapeString(title), h, js, redirectMarker))
}

// IsRedirect reports whether the page at file is a forwarding page.
func IsRedirect(file string) bool {
	info, err := os.Stat(file)
	if err != nil || info.Size() > 2048 {
		return false
	}
	data, err := os.ReadFile(file)
	return err == nil && bytes.Contains(data, []byte(redirectMarker))
}

// saveHistory writes PagesFile when it changed.
func saveHistory(bookDir string, h history) error {
	file := filepath.Join(bookDir, PagesFile)
	data := h.bytes()
	if old, err := os.ReadFile(file); err == nil && bytes.Equal(old, data) {
		return nil
	}
	return fsutil.WriteFile(file, data)
}
