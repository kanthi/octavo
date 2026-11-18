// Package xhtml turns chapters into well-formed XHTML and holds what the
// single-file formats (EPUB and PDF) share: the content stylesheet, link
// resolution and the output file name.
//
// A chapter may contain raw HTML written by the author, which is often not
// valid XML ("<br>", "&nbsp;", unclosed tags). So the chapter is rendered to
// HTML, parsed the way a browser would, and written out again as XML. The
// result is valid in an EPUB and still reads correctly as HTML.
package xhtml

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/markdown"
)

// ContentCSS styles what chapters contain: text, code, tables, callouts.
//
//go:embed content.css
var ContentCSS string

// Body renders a chapter as a well-formed XHTML fragment. visit, when not
// nil, is called for every element before it is written and may change its
// attributes (see Attr and SetAttr).
func Body(doc *markdown.Document, opts markdown.RenderOptions, visit func(el *html.Node)) ([]byte, error) {
	var src bytes.Buffer
	if err := doc.Render(&src, opts); err != nil {
		return nil, err
	}
	nodes, err := html.ParseFragment(&src, &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body})
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	for _, n := range nodes {
		write(&out, n, visit)
	}
	return out.Bytes(), nil
}

// FigurePrefix is what goes before a figure's number in a chapter: "3."
// makes "Figure 3.1". Unnumbered chapters number their figures 1, 2, 3.
func FigurePrefix(ch *book.Chapter, numbered bool) string {
	if numbered && ch.Number > 0 {
		return fmt.Sprintf("%d.", ch.Number)
	}
	return ""
}

// Attr returns the value of an attribute, or "" when the element has none.
func Attr(el *html.Node, key string) string {
	for _, a := range el.Attr {
		if a.Namespace == "" && a.Key == key {
			return a.Val
		}
	}
	return ""
}

// SetAttr changes an attribute, adding it when the element has none.
func SetAttr(el *html.Node, key, val string) {
	for i, a := range el.Attr {
		if a.Namespace == "" && a.Key == key {
			el.Attr[i].Val = val
			return
		}
	}
	el.Attr = append(el.Attr, html.Attribute{Key: key, Val: val})
}

// SetAttrNS is SetAttr for an attribute with a prefix, such as epub:type.
func SetAttrNS(el *html.Node, prefix, key, val string) {
	for i, a := range el.Attr {
		if a.Namespace == prefix && a.Key == key {
			el.Attr[i].Val = val
			return
		}
	}
	el.Attr = append(el.Attr, html.Attribute{Namespace: prefix, Key: key, Val: val})
}

// HasClass reports whether the element's class list contains name.
func HasClass(el *html.Node, name string) bool {
	for _, c := range strings.Fields(Attr(el, "class")) {
		if c == name {
			return true
		}
	}
	return false
}

// Elements that never have content or a closing tag.
var void = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true, "img": true,
	"input": true, "link": true, "meta": true, "source": true, "track": true, "wbr": true,
}

var xmlName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

var namespaces = map[string]string{"svg": "http://www.w3.org/2000/svg", "math": "http://www.w3.org/1998/Math/MathML"}

func write(w *bytes.Buffer, n *html.Node, visit func(*html.Node)) {
	switch n.Type {
	case html.TextNode:
		w.WriteString(html.EscapeString(n.Data))
	case html.ElementNode:
		// Scripts do nothing in a book file, and their text is not valid XML.
		if n.Data == "script" || !xmlName.MatchString(n.Data) {
			return
		}
		if visit != nil {
			visit(n)
		}
		w.WriteString("<" + n.Data)
		seen := map[string]bool{}
		for _, a := range n.Attr {
			key := a.Key
			if a.Namespace != "" {
				key = a.Namespace + ":" + a.Key
			}
			if seen[key] || !xmlName.MatchString(a.Key) || key == "xmlns" || strings.HasPrefix(key, "xmlns:") {
				continue
			}
			seen[key] = true
			w.WriteString(" " + key + `="` + html.EscapeString(a.Val) + `"`)
		}
		// Inline SVG and MathML must name their namespace in XML.
		if ns := namespaces[n.Namespace]; ns != "" && (n.Parent == nil || n.Parent.Namespace != n.Namespace) {
			w.WriteString(` xmlns="` + ns + `"`)
			if n.Namespace == "svg" {
				w.WriteString(` xmlns:xlink="http://www.w3.org/1999/xlink"`)
			}
		}
		if n.FirstChild == nil && (n.Namespace != "" || void[n.Data]) {
			w.WriteString(" />")
			return
		}
		w.WriteString(">")
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			write(w, c, visit)
		}
		w.WriteString("</" + n.Data + ">")
	}
}

// Target is where a link or image destination points.
type Target struct {
	Chapter  *book.Chapter // the chapter it points to, or nil
	Index    int           // that chapter's position in the book
	Path     string        // the file it points to, relative to the book folder
	Fragment string        // the part after "#"
}

// Resolver finds what the destinations written in chapters point to.
type Resolver struct {
	chapters map[string]int
	book     *book.Book
}

func NewResolver(b *book.Book) *Resolver {
	r := &Resolver{chapters: map[string]int{}, book: b}
	for i, ch := range b.Chapters {
		r.chapters[strings.TrimSuffix(ch.Path, ".md")] = i
	}
	return r
}

// Resolve reads a destination written in the chapter at from. It returns
// false for web addresses, absolute paths and links within the same page.
func (r *Resolver) Resolve(from, dest string) (Target, bool) {
	u, err := url.Parse(dest)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Path == "" || strings.HasPrefix(u.Path, "/") {
		return Target{}, false
	}
	t := Target{Path: path.Join(path.Dir(from), u.Path), Fragment: u.Fragment}
	// The HTML format may already have turned chapter links into .html.
	key := strings.TrimSuffix(strings.TrimSuffix(t.Path, ".md"), ".html")
	if i, ok := r.chapters[key]; ok && key != t.Path {
		t.Chapter, t.Index = r.book.Chapters[i], i
	}
	return t, true
}

// FileName is the default name of a book file: the title in lower case with
// dashes, such as "my-first-book", followed by ext.
func FileName(title, ext string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		} else {
			dash = true
		}
	}
	if b.Len() == 0 {
		b.WriteString("book")
	}
	return b.String() + ext
}

// HighlightCSS returns the code colours of one chroma style.
func HighlightCSS(style string) string {
	var css bytes.Buffer
	_ = chromahtml.New(chromahtml.WithClasses(true)).WriteCSS(&css, styles.Get(style))
	return css.String()
}

var backgroundRule = regexp.MustCompile(`background-color:\s*[^;}]+;?\s*`)

// HighlightColors is HighlightCSS without backgrounds and without the text
// colour of plain code, so the colours sit on whatever page colour the
// reading app uses.
func HighlightColors(style string) string {
	var out strings.Builder
	for _, line := range strings.Split(HighlightCSS(style), "\n") {
		if !strings.Contains(line, ".chroma .") { // skip the rules for the block itself
			continue
		}
		line = backgroundRule.ReplaceAllString(line, "")
		if !strings.Contains(line, "{ }") && !strings.Contains(line, "{}") {
			out.WriteString(line + "\n")
		}
	}
	return out.String()
}

// Label is a chapter's title with its number in front, when it has one.
func Label(ch *book.Chapter, numbered bool) string {
	if !numbered || ch.Number == 0 {
		return ch.Title
	}
	return strconv.Itoa(ch.Number) + ". " + ch.Title
}
