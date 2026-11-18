// Package html renders a book as a static website: one page per chapter.
// Every link is relative, so the site works from any folder on any host.
package html

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/format"
	"github.com/kanthi/octavo/internal/format/fonts"
	"github.com/kanthi/octavo/internal/markdown"
)

//go:embed theme
var theme embed.FS

// Settings are the keys under "format: html:" in octavo.yml.
type Settings struct {
	Theme          string            `yaml:"theme"`           // light, dark or auto
	Font           string            `yaml:"font"`            // the text font: literata, source-serif, atkinson or system
	CSS            config.StringList `yaml:"css"`             // your stylesheets, loaded after the built-in one
	Template       string            `yaml:"template"`        // your page template, replacing the built-in one
	Resources      config.StringList `yaml:"resources"`       // extra files to copy (globs), such as fonts
	NumberChapters *bool             `yaml:"number-chapters"` // default true
	SidebarNumbers *bool             `yaml:"sidebar-numbers"` // chapter numbers in the chapter list too, default false
	PageTOC        *bool             `yaml:"page-toc"`        // the "On this page" list beside each chapter, default true
	LastUpdated    *bool             `yaml:"last-updated"`    // "Last updated" under each chapter, and dates on the first page, default true
	Favicon        string            `yaml:"favicon"`         // the browser tab's icon; default: a small book
	SocialImage    string            `yaml:"social-image"`    // the picture shown when a page is shared on social media or in chat
	HighlightLight string            `yaml:"highlight-light"` // chroma style names
	HighlightDark  string            `yaml:"highlight-dark"`
	Redirects      map[string]string `yaml:"redirects"` // old page address -> chapter file (or full URL) it moved to
	Offline        *bool             `yaml:"offline"`   // readers can keep reading without a connection, default true
}

type Format struct{ s Settings }

// New is the format.Factory for HTML.
func New(node yaml.Node) (format.Format, error) {
	s := Settings{Theme: "auto", Font: "literata", HighlightLight: "github", HighlightDark: "github-dark"}
	if err := config.Decode(node, &s); err != nil {
		return nil, err
	}
	switch s.Theme {
	case "light", "dark", "auto":
	default:
		return nil, fmt.Errorf("theme must be light, dark or auto, not %q", s.Theme)
	}
	switch s.Font {
	case "literata", "source-serif", "atkinson", "system":
	default:
		return nil, fmt.Errorf("font must be literata, source-serif, atkinson or system, not %q (for any other font, set --font-body in your own stylesheet)", s.Font)
	}
	return &Format{s: s}, nil
}

type navItem struct {
	Title    string
	URL      string
	Number   int
	Active   bool
	Group    bool // a part or section; Children holds its entries
	Open     bool // groups: shown expanded
	Children []navItem
}

// Label is the chapter title with its number in front, when it has one.
func (n navItem) Label() string {
	if n.Number == 0 {
		return n.Title
	}
	return strconv.Itoa(n.Number) + ". " + n.Title
}

type page struct {
	Book        config.Book
	Root        string // relative path from this page to the site root, "" or "../"
	Theme       string
	Font        string // the book's default text font
	PageTitle   string
	Title       string
	Number      int
	First       bool
	Body        template.HTML
	Nav         []navItem
	Groups      bool // the chapter list has parts, so it gets "Expand all"
	Prev, Next  *navItem
	Headings    []markdown.Heading
	Stylesheets []string
	LiveReload  bool
	HomeURL     string
	HomeLabel   string
	Downloads   []format.Download // the book as PDF and EPUB, when built in the same run
	PageTOC     bool              // show the "On this page" list
	Dates       bool              // show "Last updated" and the book's dates
	Updated     string            // this chapter's date, YYYY-MM-DD
	Created     string            // the book's start date, YYYY-MM-DD, may be ""
	BookUpdated string            // the book's newest chapter date
	Favicon     string            // relative URL of the tab icon
	URL         string            // this page's full address, when the site's address is known
	Image       string            // full address of the social preview picture, when known
	Versions    []format.Version  // the version menu; URLs relative to this page
	Latest      *format.Version   // on an older version: the latest one, for the note at the top
	Offline     bool              // readers can keep reading without a connection
}

type searchEntry struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Text  string `json:"text"`
}

func (f *Format) Render(b *book.Book, out *format.Output, opts format.Options) error {
	tmpl, tmplSource, err := f.template(b.Dir)
	if err != nil {
		return err
	}
	siteURL := opts.SiteURL
	if siteURL == "" {
		siteURL = b.Config.Book.SiteURL
	}
	if siteURL != "" && !strings.HasSuffix(siteURL, "/") {
		siteURL += "/"
	}
	dates := f.s.LastUpdated == nil || *f.s.LastUpdated

	// Built-in assets. The stylesheet starts with the reading fonts.
	css, _ := theme.ReadFile("theme/book.css")
	fontCSS := "/* Reading fonts and the code font, bundled with the book so it works\n" +
		"   offline and from any host. A browser downloads only the fonts a page uses.\n" +
		"   All are under the SIL Open Font License; the licence texts are in\n" +
		"   assets/fonts/. */\n" + fonts.CSS("fonts/", fonts.All...) + fonts.CSS("fonts/", fonts.Mono) + "\n"
	if err := out.WriteData("assets/book.css", append([]byte(fontCSS), css...)); err != nil {
		return err
	}
	js, _ := theme.ReadFile("theme/book.js")
	if err := out.WriteData("assets/book.js", js); err != nil {
		return err
	}
	// The fonts and their licences.
	for _, name := range fonts.Names() {
		if err := out.WriteData("assets/fonts/"+name, fonts.Read(name)); err != nil {
			return err
		}
	}
	chroma := highlightCSS(f.s.HighlightLight, f.s.HighlightDark)
	if err := out.WriteData("assets/chroma.css", chroma); err != nil {
		return err
	}
	// Offline reading: a service worker at the top of the book. Not in the
	// preview, where it would only get in the way of seeing changes.
	offline := (f.s.Offline == nil || *f.s.Offline) && !opts.LiveReload
	if offline {
		shell := []string{"assets/book.css", "assets/chroma.css", "assets/book.js"}
		data, _ := json.Marshal(shell)
		sw, _ := theme.ReadFile("theme/sw.js")
		build := fmt.Sprintf("%x", format.Hash(fontCSS, css, js, chroma))
		sw = bytes.Replace(sw, []byte("{{build}}"), []byte(build), 1)
		sw = bytes.Replace(sw, []byte("{{shell}}"), data, 1)
		if err := out.WriteData("sw.js", sw); err != nil {
			return err
		}
	} else if !opts.LiveReload {
		// A build of some formats keeps the files already there; a worker
		// from an earlier build must still go.
		if err := os.Remove(filepath.Join(out.Dir, "sw.js")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	// The book's own files: stylesheets, listed resources, images used in chapters.
	copied := map[string]bool{}
	copyLocal := func(rel, usedBy string) {
		rel = path.Clean(rel)
		if copied[rel] {
			return
		}
		copied[rel] = true
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			fmt.Fprintf(opts.Log, "warning: %s: %s is outside the book folder, not copied\n", usedBy, rel)
			return
		}
		if err := out.Copy(filepath.Join(b.Dir, filepath.FromSlash(rel)), rel); err != nil {
			fmt.Fprintf(opts.Log, "warning: %s: %v\n", usedBy, err)
		}
	}
	for _, css := range f.s.CSS {
		copyLocal(filepath.ToSlash(css), config.FileName)
	}
	favicon := "assets/favicon.svg"
	if f.s.Favicon != "" {
		favicon = filepath.ToSlash(f.s.Favicon)
		copyLocal(favicon, config.FileName)
	} else if err := out.WriteData(favicon, defaultFavicon); err != nil {
		return err
	}
	socialImage := ""
	if f.s.SocialImage != "" {
		copyLocal(filepath.ToSlash(f.s.SocialImage), config.FileName)
		if siteURL != "" {
			socialImage = siteURL + filepath.ToSlash(f.s.SocialImage)
		}
	}
	for _, pattern := range f.s.Resources {
		matches, err := filepath.Glob(filepath.Join(b.Dir, pattern))
		if err != nil {
			return fmt.Errorf("resources %q: %w", pattern, err)
		}
		for _, m := range matches {
			if info, err := os.Stat(m); err == nil && !info.IsDir() {
				rel, _ := filepath.Rel(b.Dir, m)
				copyLocal(filepath.ToSlash(rel), config.FileName)
			}
		}
	}

	numbered := f.s.NumberChapters == nil || *f.s.NumberChapters
	// The chapter list shows titles only, unless asked for numbers too; the
	// chapter's own heading keeps its number either way.
	listNumbers := numbered && f.s.SidebarNumbers != nil && *f.s.SidebarNumbers
	headingNumber := func(ch *book.Chapter) int {
		if numbered {
			return ch.Number
		}
		return 0
	}
	item := func(ch *book.Chapter, title, root string) navItem {
		it := navItem{Title: title, URL: root + outPath(ch.Path)}
		if listNumbers {
			it.Number = ch.Number
		}
		return it
	}
	// sidebar builds the chapter list as seen from one page. A group is
	// expanded only when it holds the current page, so a long book shows its
	// parts plus where the reader is; "Expand all" in the list opens the rest.
	var sidebar func(nodes []*book.Node, current *book.Chapter, root string) ([]navItem, bool)
	sidebar = func(nodes []*book.Node, current *book.Chapter, root string) ([]navItem, bool) {
		var items []navItem
		holdsCurrent := false
		for _, n := range nodes {
			if n.Chapter != nil {
				it := item(n.Chapter, n.Title, root)
				it.Active = n.Chapter == current
				holdsCurrent = holdsCurrent || it.Active
				items = append(items, it)
				continue
			}
			children, has := sidebar(n.Children, current, root)
			holdsCurrent = holdsCurrent || has
			items = append(items, navItem{Title: n.Title, Group: true, Open: has, Children: children})
		}
		return items, holdsCurrent
	}

	// Everything a page shows besides its own chapter: the book's details, the
	// settings, the template, and the whole chapter list (titles, numbers,
	// order, groups). A page is rendered again only when this or its chapter's
	// source changes. So editing a chapter's text redoes one page, while
	// renaming a chapter redoes every page, because every sidebar shows it.
	settings, _ := json.Marshal(f.s)
	bookInfo, _ := json.Marshal(b.Config.Book)
	shell := format.Hash(settings, bookInfo, tmplSource, opts.LiveReload, opts.HomeURL, opts.HomeLabel, opts.Downloads, opts.Versions, siteURL, b.Created, b.Updated)
	for _, ch := range b.Chapters {
		shell = format.Hash(shell, ch.Path, ch.Title, ch.Number)
	}

	var index []searchEntry
	hasIndex := false
	for i, ch := range b.Chapters {
		target := outPath(ch.Path)
		hasIndex = hasIndex || target == "index.html"
		root := strings.Repeat("../", strings.Count(target, "/"))

		// Images, and their versions for dark pages: "name-dark.svg" beside
		// "name.svg" is used when the page is dark.
		dark := map[string]string{}
		for _, img := range ch.Doc.Images {
			p, ok := localPath(img)
			if !ok {
				continue
			}
			copyLocal(path.Join(path.Dir(ch.Path), p), ch.Path)
			ext := path.Ext(p)
			darkP := strings.TrimSuffix(p, ext) + "-dark" + ext
			darkRel := path.Join(path.Dir(ch.Path), darkP)
			if _, err := os.Stat(filepath.Join(b.Dir, filepath.FromSlash(darkRel))); err == nil && filepath.IsLocal(filepath.FromSlash(darkRel)) {
				copyLocal(darkRel, ch.Path)
				dark[img] = strings.Replace(img, p, darkP, 1)
			}
		}
		index = append(index, searchEntry{Title: item(ch, ch.Title, "").Label(), URL: target, Text: ch.Doc.PlainText()})
		fingerprint := format.Hash(shell, ch.Hash, i, ch.Updated, dark)
		if out.Fresh(target, fingerprint) {
			continue
		}
		ch.Doc.RewriteLinks(chapterLink)
		var body bytes.Buffer
		prefix := ""
		if numbered && ch.Number > 0 {
			prefix = strconv.Itoa(ch.Number) + "."
		}
		if err := ch.Doc.Render(&body, markdown.RenderOptions{Format: "html", FigurePrefix: prefix, Dark: dark, Lazy: true}); err != nil {
			return fmt.Errorf("chapter %s: %w", ch.Path, err)
		}

		p := page{
			Book: b.Config.Book, Root: root, Theme: f.s.Theme, Font: f.s.Font,
			PageTitle: ch.Title, Title: ch.Title, Number: headingNumber(ch), First: i == 0,
			Body:     template.HTML(headingLinks(body.String())),
			Headings: ch.Doc.HeadingsFor("html"), LiveReload: opts.LiveReload,
			PageTOC: f.s.PageTOC == nil || *f.s.PageTOC,
			Dates:   dates, Updated: ch.Updated, Created: b.Created, BookUpdated: b.Updated,
			Favicon: root + favicon, Image: socialImage, Offline: offline,
			Stylesheets: []string{root + "assets/book.css", root + "assets/chroma.css"},
		}
		if ch.Title != b.Config.Book.Title {
			p.PageTitle = ch.Title + " – " + b.Config.Book.Title
		}
		if siteURL != "" {
			p.URL = siteURL + target
		}
		for _, css := range f.s.CSS {
			p.Stylesheets = append(p.Stylesheets, root+filepath.ToSlash(css))
		}
		p.Nav, _ = sidebar(b.Nav, ch, root)
		for _, it := range p.Nav {
			p.Groups = p.Groups || it.Group
		}
		if opts.HomeURL != "" {
			p.HomeURL, p.HomeLabel = root+opts.HomeURL, opts.HomeLabel
		}
		for _, d := range opts.Downloads {
			p.Downloads = append(p.Downloads, format.Download{Label: d.Label, URL: root + d.URL})
		}
		current := false
		for _, v := range opts.Versions {
			v.URL = root + v.URL
			p.Versions = append(p.Versions, v)
			current = current || v.Current && v.Latest
		}
		if len(p.Versions) > 0 && !current {
			p.Latest = &p.Versions[0]
		}
		if i > 0 {
			prev := item(b.Chapters[i-1], b.Chapters[i-1].Title, root)
			p.Prev = &prev
		}
		if i < len(b.Chapters)-1 {
			next := item(b.Chapters[i+1], b.Chapters[i+1].Title, root)
			p.Next = &next
		}

		var html bytes.Buffer
		if err := tmpl.Execute(&html, p); err != nil {
			return fmt.Errorf("chapter %s: %w", ch.Path, err)
		}
		if err := out.Write(target, fingerprint, html.Bytes()); err != nil {
			return err
		}
	}

	// Old addresses of moved chapters forward to their new place. The list of
	// addresses is saved by real builds only, not by the preview, where a
	// half-done rename would otherwise be remembered.
	warn := func(msg string, args ...any) {
		if !opts.LiveReload {
			fmt.Fprintf(opts.Log, "warning: "+msg+"\n", args...)
		}
	}
	pages, err := f.redirects(b, out, warn)
	if err != nil {
		return err
	}
	if !opts.LiveReload {
		if err := saveHistory(b.Dir, pages); err != nil {
			return fmt.Errorf("%s: %w", PagesFile, err)
		}
	}

	if !hasIndex {
		redirect := fmt.Sprintf("<!doctype html>\n<meta charset=\"utf-8\">\n<meta http-equiv=\"refresh\" content=\"0; url=%s\">\n<a href=\"%[1]s\">Open the book</a>\n",
			template.HTMLEscapeString(outPath(b.Chapters[0].Path)))
		if err := out.WriteData("index.html", []byte(redirect)); err != nil {
			return err
		}
	}
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	if err := out.WriteData("search.json", data); err != nil {
		return err
	}
	if opts.InSite {
		return nil // the library writes the sitemap and the 404 page for the whole site
	}
	if siteURL != "" {
		var pages []string
		for _, ch := range b.Chapters {
			pages = append(pages, outPath(ch.Path))
		}
		if err := out.WriteData("sitemap.xml", Sitemap(siteURL, pages)); err != nil {
			return err
		}
	}
	return out.WriteData("404.html", NotFound(b.Config.Book.Title, siteURL))
}

// headingLinks adds a "#" link to every section heading, for readers to copy
// a link to that place.
var sectionHeading = regexp.MustCompile(`(<h([2-4]) id="([^"]+)">)(.*?)(</h[2-4]>)`)

func headingLinks(body string) string {
	return sectionHeading.ReplaceAllString(body, `$1$4<a class="heading-link" href="#$3" aria-label="Link to this section">#</a>$5`)
}

// Sitemap lists a site's pages for search engines.
func Sitemap(siteURL string, pages []string) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range pages {
		b.WriteString("  <url><loc>")
		template.HTMLEscape(&b, []byte(siteURL+p))
		b.WriteString("</loc></url>\n")
	}
	b.WriteString("</urlset>\n")
	return b.Bytes()
}

// NotFound is a 404 page. Hosts serve it at any address, so it cannot use
// relative links: it links to the site's address when known, else to "/".
func NotFound(title, siteURL string) []byte {
	home := siteURL
	if home == "" {
		home = "/"
	}
	return []byte(fmt.Sprintf(`<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Page not found – %[1]s</title>
<style>
  body { margin: 0; display: grid; place-items: center; min-height: 100vh; background: #fff; color: #1d2330; font: 1.05rem/1.6 system-ui, sans-serif; text-align: center; }
  @media (prefers-color-scheme: dark) { body { background: #14171d; color: #e4e7ee; } a { color: #7ea6ff; } }
  main { padding: 2rem; } h1 { margin: 0 0 .5rem; font-size: 1.6rem; } a { color: #2456c9; }
</style>
<main>
  <h1>Page not found</h1>
  <p>This page is not part of %[1]s, or it has moved.</p>
  <p><a href="%[2]s">Go to the start of the book</a></p>
</main>
</html>
`, template.HTMLEscapeString(title), template.HTMLEscapeString(home)))
}

// defaultFavicon is a small book, used when the book names no icon of its own.
var defaultFavicon = []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect x="5" y="3" width="22" height="26" rx="3" fill="#2456c9"/><rect x="9" y="3" width="2" height="26" fill="#fff" opacity=".35"/><rect x="14" y="10" width="9" height="2" rx="1" fill="#fff"/><rect x="14" y="15" width="7" height="2" rx="1" fill="#fff"/></svg>
`)

func (f *Format) template(bookDir string) (*template.Template, []byte, error) {
	src, _ := theme.ReadFile("theme/page.html")
	name := "built-in page template"
	if f.s.Template != "" {
		name = f.s.Template
		var err error
		if src, err = os.ReadFile(filepath.Join(bookDir, f.s.Template)); err != nil {
			return nil, nil, fmt.Errorf("template: %w", err)
		}
	}
	t, err := template.New("page").Parse(string(src))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", name, err)
	}
	return t, src, nil
}

// outPath maps a chapter source path to its page path: a/b.md -> a/b.html.
func outPath(chapter string) string {
	return strings.TrimSuffix(chapter, ".md") + ".html"
}

// localPath returns the file path of a destination that points inside the
// book, and false for web addresses and absolute paths.
func localPath(dest string) (string, bool) {
	u, err := url.Parse(dest)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Path == "" || strings.HasPrefix(u.Path, "/") {
		return "", false
	}
	return u.Path, true
}

// chapterLink turns a link to another chapter's .md file into its .html page.
func chapterLink(dest string) string {
	p, ok := localPath(dest)
	if !ok || !strings.HasSuffix(p, ".md") {
		return dest
	}
	u, _ := url.Parse(dest)
	u.Path = outPath(p)
	return u.String()
}

var highlightCache = map[string][]byte{}

// highlightCSS builds the code colours, one set of rules per theme. Each set
// is scoped to its theme so the light colours never leak into dark mode.
func highlightCSS(light, dark string) []byte {
	if css, ok := highlightCache[light+"|"+dark]; ok {
		return css
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	var out bytes.Buffer
	for _, pair := range [][2]string{{"light", light}, {"dark", dark}} {
		theme, style := pair[0], pair[1]
		var css bytes.Buffer
		_ = formatter.WriteCSS(&css, styles.Get(style))
		for _, line := range strings.Split(css.String(), "\n") {
			if strings.Contains(line, ".chroma") {
				out.WriteString(strings.ReplaceAll(line, ".chroma", `[data-theme="`+theme+`"] .chroma`) + "\n")
			}
		}
	}
	highlightCache[light+"|"+dark] = out.Bytes()
	return out.Bytes()
}
