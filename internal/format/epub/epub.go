// Package epub renders a book as one EPUB 3 file, for e-readers.
//
// The file follows EPUB 3.3 and stays readable by older EPUB 2 apps:
//   - a navigation document with the table of contents and landmarks, plus an
//     NCX table of contents for EPUB 2 apps
//   - a cover (the book's own image, or a generated one), a title page, a
//     contents page and a page for every part and section
//   - semantic markup (epub:type and ARIA roles) for chapters, footnotes and
//     front matter, so reading apps can show footnotes as pop-ups and screen
//     readers can announce the structure
//   - accessibility metadata, as asked for by EPUB Accessibility 1.1
//   - a stylesheet that leaves font, size and colours to the reader's app
package epub

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	_ "embed"
	"fmt"
	"hash/fnv"
	"html"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	nethtml "golang.org/x/net/html"
	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/format"
	"github.com/kanthi/octavo/internal/format/xhtml"
	"github.com/kanthi/octavo/internal/markdown"
)

//go:embed epub.css
var epubCSS string

// Settings are the keys under "format: epub:" in octavo.yml.
type Settings struct {
	OutputFile     string            `yaml:"output-file"`     // default: the book title, such as my-book.epub
	Cover          string            `yaml:"cover"`           // your cover image (PNG or JPEG); "none" for no cover; default: a generated cover
	CSS            config.StringList `yaml:"css"`             // your stylesheets, added after the built-in one
	Highlight      string            `yaml:"highlight"`       // chroma style for code on light pages
	HighlightDark  string            `yaml:"highlight-dark"`  // chroma style for code on dark pages
	NumberChapters *bool             `yaml:"number-chapters"` // default true
}

type Format struct{ s Settings }

// New is the format.Factory for EPUB.
func New(node yaml.Node) (format.Format, error) {
	s := Settings{Highlight: "github", HighlightDark: "github-dark"}
	if err := config.Decode(node, &s); err != nil {
		return nil, err
	}
	if s.OutputFile != "" && !strings.HasSuffix(s.OutputFile, ".epub") {
		s.OutputFile += ".epub"
	}
	return &Format{s: s}, nil
}

var esc = html.EscapeString

// File is the name of the EPUB in the output folder.
func (f *Format) File(b *book.Book) string {
	if f.s.OutputFile != "" {
		return f.s.OutputFile
	}
	return xhtml.FileName(b.Config.Book.Title, ".epub")
}

// item is one file inside the EPUB, listed in its manifest.
type item struct {
	id, href, mediaType, properties string
	data                            []byte
	spine                           bool
}

// entry is one line of the table of contents.
type entry struct {
	label    string
	href     string // relative to the OEBPS folder
	group    bool
	children []*entry
}

const xhtmlType = "application/xhtml+xml"

func (f *Format) Render(b *book.Book, out *format.Output, opts format.Options) error {
	info := b.Config.Book
	numbered := f.s.NumberChapters == nil || *f.s.NumberChapters
	resolver := xhtml.NewResolver(b)
	chapterFile := func(i int) string { return fmt.Sprintf("ch%03d.xhtml", i+1) }
	index := map[*book.Chapter]int{}
	for i, ch := range b.Chapters {
		index[ch] = i
	}

	var items []item
	images := map[string]string{} // source path in the book -> href in the EPUB
	addImage := func(rel, usedBy string) (string, bool) {
		if href, ok := images[rel]; ok {
			return href, href != ""
		}
		images[rel] = ""
		data, err := os.ReadFile(filepath.Join(b.Dir, filepath.FromSlash(rel)))
		if !filepath.IsLocal(filepath.FromSlash(rel)) || err != nil {
			fmt.Fprintf(opts.Log, "warning: %s: image %s not found, left out of the EPUB\n", usedBy, rel)
			return "", false
		}
		href := "images/" + rel
		images[rel] = href
		items = append(items, item{id: fmt.Sprintf("img%d", len(images)), href: href, mediaType: mediaType(rel), data: data})
		return href, true
	}

	// The stylesheet: built-in rules, code colours for light and dark pages,
	// then the book's own.
	css := epubCSS + "\n/* Code colours */\n" + xhtml.HighlightColors(f.s.Highlight) +
		"@media (prefers-color-scheme: dark) {\n" + xhtml.HighlightColors(f.s.HighlightDark) + "}\n"
	for _, file := range f.s.CSS {
		data, err := os.ReadFile(filepath.Join(b.Dir, file))
		if err != nil {
			return fmt.Errorf("css: %w", err)
		}
		css += "\n" + string(data)
	}
	items = append(items, item{id: "css", href: "css/book.css", mediaType: "text/css", data: []byte(css)})

	// Pages sit in text/, so everything else is one folder up.
	page := func(title, bodyAttrs, body string) []byte {
		return []byte(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="` + esc(info.Lang) + `" xml:lang="` + esc(info.Lang) + `">
<head>
<meta charset="utf-8" />
<title>` + esc(title) + `</title>
<link rel="stylesheet" type="text/css" href="../css/book.css" />
</head>
<body` + bodyAttrs + `>
` + body + `
</body>
</html>
`)
	}

	// Cover: the book's own image, or one drawn from the title.
	hasCover := f.s.Cover != "none"
	if hasCover {
		href := "images/octavo-cover.svg"
		if f.s.Cover != "" {
			var ok bool
			if href, ok = addImage(filepath.ToSlash(filepath.Clean(f.s.Cover)), config.FileName); !ok {
				return fmt.Errorf("cover %s: file not found in the book folder", f.s.Cover)
			}
			items[len(items)-1].properties = "cover-image"
		} else {
			items = append(items, item{id: "cover-image", href: href, mediaType: "image/svg+xml", properties: "cover-image", data: coverSVG(info)})
		}
		items = append(items, item{id: "cover", href: "text/cover.xhtml", mediaType: xhtmlType, spine: true,
			data: page(info.Title, ` class="cover" epub:type="cover"`,
				`<img src="../`+esc(escapePath(href))+`" alt="Cover: `+esc(info.Title)+`" />`)})
	}

	title := `<section class="title-page" epub:type="titlepage" role="doc-titlepage">` + "\n<h1>" + esc(info.Title) + "</h1>\n"
	for _, line := range []string{info.Author, info.Date} {
		if line != "" {
			title += "<p>" + esc(line) + "</p>\n"
		}
	}
	if info.Description != "" {
		title += `<p class="description">` + esc(info.Description) + "</p>\n"
	}
	items = append(items, item{id: "title", href: "text/title.xhtml", mediaType: xhtmlType, spine: true,
		data: page(info.Title, ` epub:type="frontmatter"`, title+"</section>")})

	// The contents page comes third; its text is written once the chapters are known.
	navAt := len(items)
	items = append(items, item{id: "nav", href: "nav.xhtml", mediaType: xhtmlType, properties: "nav", spine: true})

	// Chapters and group pages, in reading order.
	groups := 0
	var walk func(nodes []*book.Node) ([]*entry, error)
	walk = func(nodes []*book.Node) ([]*entry, error) {
		var entries []*entry
		for _, n := range nodes {
			if n.Chapter == nil {
				if len(n.Children) == 0 {
					continue
				}
				// Every part and section gets a page, so the table of
				// contents can point at it.
				groups++
				file := fmt.Sprintf("part%03d.xhtml", groups)
				items = append(items, item{id: fmt.Sprintf("part%03d", groups), href: "text/" + file, mediaType: xhtmlType, spine: true,
					data: page(n.Title, ` epub:type="bodymatter"`,
						`<section class="part-page" epub:type="part" role="doc-part" aria-label="`+esc(n.Title)+`">`+"\n<h1>"+esc(n.Title)+"</h1>\n</section>")})
				children, err := walk(n.Children)
				if err != nil {
					return nil, err
				}
				entries = append(entries, &entry{label: n.Title, href: "text/" + file, group: true, children: children})
				continue
			}
			ch, i := n.Chapter, index[n.Chapter]
			body, err := xhtml.Body(ch.Doc, markdown.RenderOptions{Format: "epub", FigurePrefix: xhtml.FigurePrefix(ch, numbered)}, func(el *nethtml.Node) {
				// Footnote ids from the Markdown renderer contain ":", which
				// is not allowed in an XML id.
				if id := xhtml.Attr(el, "id"); strings.Contains(id, ":") {
					xhtml.SetAttr(el, "id", strings.ReplaceAll(id, ":", "-"))
				}
				switch el.Data {
				case "a":
					href := xhtml.Attr(el, "href")
					if strings.HasPrefix(href, "#") {
						xhtml.SetAttr(el, "href", strings.ReplaceAll(href, ":", "-"))
					} else if t, ok := resolver.Resolve(ch.Path, href); ok && t.Chapter != nil {
						// A link to another chapter points at its page in the EPUB.
						href = chapterFile(t.Index)
						if t.Fragment != "" {
							href += "#" + t.Fragment
						}
						xhtml.SetAttr(el, "href", href)
					}
					// Marking footnotes lets reading apps show them as pop-ups.
					if xhtml.HasClass(el, "footnote-ref") {
						xhtml.SetAttrNS(el, "epub", "type", "noteref")
					}
				case "div":
					if xhtml.HasClass(el, "footnotes") {
						xhtml.SetAttrNS(el, "epub", "type", "footnotes")
					}
				case "li":
					if strings.HasPrefix(xhtml.Attr(el, "id"), "fn-") {
						xhtml.SetAttrNS(el, "epub", "type", "footnote")
						xhtml.SetAttr(el, "role", "doc-footnote")
					}
				case "img":
					if t, ok := resolver.Resolve(ch.Path, xhtml.Attr(el, "src")); ok {
						if href, ok := addImage(t.Path, ch.Path); ok {
							xhtml.SetAttr(el, "src", "../"+escapePath(href))
						}
					}
					if xhtml.Attr(el, "alt") == "" {
						xhtml.SetAttr(el, "alt", "")
					}
				}
			})
			if err != nil {
				return nil, fmt.Errorf("chapter %s: %w", ch.Path, err)
			}
			id := fmt.Sprintf("ch%03d", i+1)
			heading := `<h1 id="` + id + `-title">` + esc(ch.Title) + "</h1>"
			label := n.Title
			if numbered && ch.Number > 0 {
				heading = fmt.Sprintf(`<h1 id="%s-title"><span class="chapter-number">Chapter %d</span> %s</h1>`, id, ch.Number, esc(ch.Title))
				label = fmt.Sprintf("%d. %s", ch.Number, n.Title)
			}
			it := item{id: id, href: "text/" + chapterFile(i), mediaType: xhtmlType, spine: true,
				data: page(ch.Title, ` epub:type="bodymatter"`,
					`<section class="chapter" epub:type="chapter" role="doc-chapter" aria-labelledby="`+id+`-title">`+"\n"+heading+"\n"+string(body)+"\n</section>")}
			if bytes.Contains(body, []byte("<svg")) {
				it.properties = "svg"
			}
			items = append(items, it)
			entries = append(entries, &entry{label: label, href: it.href})
		}
		return entries, nil
	}
	toc, err := walk(b.Nav)
	if err != nil {
		return err
	}

	bookID := identifier(info)
	items[navAt].data = navDocument(info, toc, hasCover, "text/"+chapterFile(0))
	items = append(items, item{id: "ncx", href: "toc.ncx", mediaType: "application/x-dtbncx+xml", data: ncx(info, bookID, toc)})

	data, err := pack(items, f.opf(info, bookID, items))
	if err != nil {
		return err
	}
	return out.WriteData(f.File(b), data)
}

// navDocument writes the EPUB 3 navigation file: the table of contents that
// reading apps show in their own menu, and the landmarks (cover, contents,
// start of the text). It is also a page of the book.
func navDocument(info config.Book, toc []*entry, hasCover bool, start string) []byte {
	var list func(entries []*entry, indent string) string
	list = func(entries []*entry, indent string) string {
		var s strings.Builder
		for _, e := range entries {
			class := ""
			if e.group {
				class = ` class="group"`
			}
			s.WriteString(indent + `<li><a` + class + ` href="` + esc(e.href) + `">` + esc(e.label) + "</a>")
			if len(e.children) > 0 {
				s.WriteString("\n" + indent + "<ol>\n" + list(e.children, indent+"  ") + indent + "</ol>\n" + indent)
			}
			s.WriteString("</li>\n")
		}
		return s.String()
	}
	landmarks := ""
	if hasCover {
		landmarks += `    <li><a epub:type="cover" href="text/cover.xhtml">Cover</a></li>` + "\n"
	}
	landmarks += `    <li><a epub:type="titlepage" href="text/title.xhtml">Title page</a></li>
    <li><a epub:type="toc" href="nav.xhtml">Contents</a></li>
    <li><a epub:type="bodymatter" href="` + start + `">Start of the book</a></li>
`
	return []byte(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="` + esc(info.Lang) + `" xml:lang="` + esc(info.Lang) + `">
<head>
<meta charset="utf-8" />
<title>Contents</title>
<link rel="stylesheet" type="text/css" href="css/book.css" />
</head>
<body epub:type="frontmatter">
<nav epub:type="toc" role="doc-toc" id="toc" aria-labelledby="toc-title">
<h1 id="toc-title">Contents</h1>
<ol>
` + list(toc, "  ") + `</ol>
</nav>
<nav epub:type="landmarks" id="landmarks" hidden="hidden">
  <h2>Guide</h2>
  <ol>
` + landmarks + `  </ol>
</nav>
</body>
</html>
`)
}

// ncx writes the EPUB 2 table of contents, for older reading apps.
func ncx(info config.Book, bookID string, toc []*entry) []byte {
	order, depth := 0, 0
	var points func(entries []*entry, level int) string
	points = func(entries []*entry, level int) string {
		if level > depth && len(entries) > 0 {
			depth = level
		}
		var s strings.Builder
		for _, e := range entries {
			order++
			fmt.Fprintf(&s, `<navPoint id="np%d" playOrder="%d"><navLabel><text>%s</text></navLabel><content src="%s" />`,
				order, order, esc(e.label), esc(e.href))
			if len(e.children) > 0 {
				s.WriteString("\n" + points(e.children, level+1))
			}
			s.WriteString("</navPoint>\n")
		}
		return s.String()
	}
	body := points(toc, 1)
	return []byte(`<?xml version="1.0" encoding="utf-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1" xml:lang="` + esc(info.Lang) + `">
<head>
<meta name="dtb:uid" content="` + bookID + `" />
<meta name="dtb:depth" content="` + fmt.Sprint(depth) + `" />
<meta name="dtb:totalPageCount" content="0" />
<meta name="dtb:maxPageNumber" content="0" />
</head>
<docTitle><text>` + esc(info.Title) + `</text></docTitle>
<navMap>
` + body + `</navMap>
</ncx>
`)
}

// identifier is the book's unique id. It is made from the title and author,
// so it stays the same between builds of the same book.
func identifier(info config.Book) string {
	sum := sha1.Sum([]byte("octavo\x00" + info.Title + "\x00" + info.Author))
	sum[6] = sum[6]&0x0f | 0x50 // UUID version 5
	sum[8] = sum[8]&0x3f | 0x80 // UUID variant
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

// opf writes the package document: the book's details, every file, and the
// reading order.
func (f *Format) opf(info config.Book, bookID string, items []item) []byte {
	var s strings.Builder
	s.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id" xml:lang="` + esc(info.Lang) + `">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:identifier id="book-id">` + bookID + `</dc:identifier>
<dc:title>` + esc(info.Title) + `</dc:title>
<dc:language>` + esc(info.Lang) + `</dc:language>
`)
	if info.Author != "" {
		s.WriteString(`<dc:creator id="creator">` + esc(info.Author) + "</dc:creator>\n" +
			`<meta refines="#creator" property="role" scheme="marc:relators">aut</meta>` + "\n")
	}
	if info.Description != "" {
		s.WriteString("<dc:description>" + esc(info.Description) + "</dc:description>\n")
	}
	if d, err := time.Parse("2006-01-02", info.Date); err == nil {
		s.WriteString("<dc:date>" + d.Format("2006-01-02") + "</dc:date>\n")
	}
	s.WriteString(`<meta property="dcterms:modified">` + time.Now().UTC().Format("2006-01-02T15:04:05Z") + "</meta>\n")
	// Accessibility metadata (EPUB Accessibility 1.1). Only what is true of
	// every book Octavo builds is claimed here.
	s.WriteString(`<meta property="schema:accessMode">textual</meta>
<meta property="schema:accessMode">visual</meta>
<meta property="schema:accessibilityFeature">structuralNavigation</meta>
<meta property="schema:accessibilityFeature">tableOfContents</meta>
<meta property="schema:accessibilityFeature">readingOrder</meta>
<meta property="schema:accessibilityFeature">displayTransformability</meta>
<meta property="schema:accessibilityHazard">unknown</meta>
<meta property="schema:accessibilitySummary">Reflowable text with a table of contents and headings for navigation. Font, text size and colours can be changed in the reading app. Images have text descriptions where the author wrote them.</meta>
`)
	for _, it := range items {
		if it.properties == "cover-image" {
			s.WriteString(`<meta name="cover" content="` + it.id + `" />` + "\n") // how EPUB 2 apps find the cover
		}
	}
	s.WriteString("</metadata>\n<manifest>\n")
	for _, it := range items {
		s.WriteString(`<item id="` + it.id + `" href="` + esc(escapePath(it.href)) + `" media-type="` + it.mediaType + `"`)
		if it.properties != "" {
			s.WriteString(` properties="` + it.properties + `"`)
		}
		s.WriteString(" />\n")
	}
	s.WriteString("</manifest>\n" + `<spine toc="ncx">` + "\n")
	for _, it := range items {
		if it.spine {
			s.WriteString(`<itemref idref="` + it.id + `" />` + "\n")
		}
	}
	s.WriteString("</spine>\n</package>\n")
	return []byte(s.String())
}

// coverSVG draws a plain cover from the title and author: reading apps show
// the cover in their library, and a book without one looks broken there.
func coverSVG(info config.Book) []byte {
	h := fnv.New32a()
	h.Write([]byte(info.Title))
	hue := h.Sum32() % 360

	// Break the title into lines of at most about 14 letters.
	var lines []string
	line := ""
	for _, word := range strings.Fields(info.Title) {
		if line != "" && len([]rune(line))+1+len([]rune(word)) > 14 {
			lines = append(lines, line)
			line = word
		} else if line == "" {
			line = word
		} else {
			line += " " + word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	longest := 1
	for _, l := range lines {
		if n := len([]rune(l)); n > longest {
			longest = n
		}
	}
	size := 1300 * 10 / (longest * 6) // the widest line fills about 1300 of 1600 units
	if size > 220 {
		size = 220
	}
	if max := 1200 / len(lines); size > max { // many lines: keep the block inside the page
		size = max
	}

	var s strings.Builder
	fmt.Fprintf(&s, `<?xml version="1.0" encoding="utf-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1600 2400" width="1600" height="2400" role="img" aria-label="%s">
<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="hsl(%d, 55%%, 34%%)" /><stop offset="1" stop-color="hsl(%d, 60%%, 16%%)" /></linearGradient></defs>
<rect width="1600" height="2400" fill="url(#g)" />
<rect x="110" y="110" width="1380" height="2180" fill="none" stroke="#ffffff" stroke-opacity=".35" stroke-width="6" />
`, esc(info.Title), hue, (hue+40)%360)
	y := 760
	for _, l := range lines {
		fmt.Fprintf(&s, `<text x="800" y="%d" text-anchor="middle" font-family="Georgia, 'Times New Roman', serif" font-size="%d" font-weight="bold" fill="#ffffff">%s</text>`+"\n", y, size, esc(l))
		y += size * 12 / 10
	}
	fmt.Fprintf(&s, `<rect x="700" y="%d" width="200" height="8" fill="#ffffff" fill-opacity=".7" />`+"\n", y-size/2)
	if info.Author != "" {
		fmt.Fprintf(&s, `<text x="800" y="2090" text-anchor="middle" font-family="Georgia, 'Times New Roman', serif" font-size="84" fill="#ffffff" fill-opacity=".9">%s</text>`+"\n", esc(info.Author))
	}
	s.WriteString("</svg>\n")
	return []byte(s.String())
}

// pack writes the EPUB container: a zip file whose first entry is an
// uncompressed "mimetype".
func pack(items []item, opf []byte) ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	add := func(name string, method uint16, data []byte) error {
		w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	if err := add("mimetype", zip.Store, []byte("application/epub+zip")); err != nil {
		return nil, err
	}
	container := `<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles>
<rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml" />
</rootfiles>
</container>
`
	if err := add("META-INF/container.xml", zip.Deflate, []byte(container)); err != nil {
		return nil, err
	}
	if err := add("OEBPS/content.opf", zip.Deflate, opf); err != nil {
		return nil, err
	}
	for _, it := range items {
		if err := add("OEBPS/"+it.href, zip.Deflate, it.data); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// escapePath makes a file path safe to use as a link.
func escapePath(p string) string { return (&url.URL{Path: p}).EscapedPath() }

func mediaType(file string) string {
	switch ext := strings.ToLower(path.Ext(file)); ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	default:
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
		return "application/octet-stream"
	}
}
