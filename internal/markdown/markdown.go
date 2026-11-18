// Package markdown parses a chapter into a format-neutral document. Formats
// (HTML now, EPUB later) decide how to turn that document into output.
package markdown

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"gopkg.in/yaml.v3"
)

// Meta is the YAML front matter at the top of a chapter.
type Meta struct {
	Title      string `yaml:"title"`
	Unnumbered bool   `yaml:"unnumbered"`
}

type Heading struct {
	Level int
	ID    string
	Text  string
	when  condition
}

// Image is one image in a chapter, as written.
type Image struct {
	Dest string
	Alt  string
}

type Document struct {
	Meta     Meta
	Title    string          // from a leading "# Heading", which is removed from the body
	Headings []Heading       // level 2 and 3 headings, for the "on this page" list (see HeadingsFor)
	Images   []string        // image destinations as written in the chapter
	Pictures []Image         // the same images, with their alt text
	Links    []string        // link destinations as written in the chapter
	IDs      map[string]bool // every id a link can point at: headings, figures, raw HTML

	source []byte
	root   ast.Node
	blocks []block // the top-level parts of the chapter, with the formats that show them
	plain  *string // PlainText, worked out once
}

type block struct {
	node ast.Node
	when condition
}

var rawID = regexp.MustCompile(`\bid="([^"]+)"`)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Footnote,
		extension.Typographer, // curly quotes, dashes, ellipses
		highlighting.NewHighlighting(highlighting.WithFormatOptions(chromahtml.WithClasses(true)), highlighting.WithCodeBlockOptions(codeLanguage)),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
		parser.WithASTTransformers(util.Prioritized(calloutTransformer{}, 500), util.Prioritized(figureTransformer{}, 600)),
	),
	goldmark.WithRendererOptions(
		gmhtml.WithUnsafe(), // book authors are trusted, so raw HTML is allowed
		renderer.WithNodeRenderers(util.Prioritized(calloutRenderer{}, 500), util.Prioritized(extraRenderer{}, 500)),
	),
)

// codeLanguage writes a highlighted code block's language on its <pre> as
// data-lang, for formats that colour code themselves (the Typst PDF).
func codeLanguage(ctx highlighting.CodeBlockContext) []chromahtml.Option {
	lang, ok := ctx.Language()
	if !ok || len(lang) == 0 {
		return nil
	}
	attr := ` data-lang="` + html.EscapeString(string(lang)) + `"`
	return []chromahtml.Option{chromahtml.WithPreWrapper(langPre(attr))}
}

// langPre is chroma's own <pre><code> wrapper with one more attribute.
type langPre string

func (p langPre) Start(code bool, style string) string {
	if code {
		return "<pre" + style + string(p) + "><code>"
	}
	return "<pre" + style + string(p) + ">"
}

func (p langPre) End(code bool) string {
	if code {
		return "</code></pre>"
	}
	return "</pre>"
}

// Parse reads one chapter.
func Parse(src []byte) (*Document, error) {
	front, body := splitFrontMatter(src)
	d := &Document{source: body, IDs: map[string]bool{}}
	if len(front) > 0 {
		if err := yaml.Unmarshal(front, &d.Meta); err != nil {
			return nil, fmt.Errorf("front matter: %w", err)
		}
	}
	d.root = md.Parser().Parse(text.NewReader(body))

	if h, ok := d.root.FirstChild().(*ast.Heading); ok && h.Level == 1 {
		d.Title = textOf(h, body)
		d.root.RemoveChild(d.root, h)
	}
	// Split the chapter into its top-level parts, noting which formats show
	// each one. Format blocks only work at the top level.
	var when condition
	for n := d.root.FirstChild(); n != nil; n = n.NextSibling() {
		if open, end := marker(n, body); open != nil {
			when = append(append(condition{}, when...), *open)
			continue
		} else if end {
			if len(when) == 0 {
				return nil, fmt.Errorf("a format block ends without starting")
			}
			when = when[:len(when)-1]
			continue
		}
		d.blocks = append(d.blocks, block{node: n, when: when})
		d.collect(n, when)
	}
	if len(when) > 0 {
		return nil, fmt.Errorf("a format block (\":::\") is not closed")
	}
	return d, nil
}

// collect notes the headings, images, links and ids inside n.
func (d *Document) collect(n ast.Node, when condition) {
	body := d.source
	_ = ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Heading:
			if id, ok := v.AttributeString("id"); ok {
				d.IDs[string(id.([]byte))] = true
				if v.Level == 2 || v.Level == 3 {
					d.Headings = append(d.Headings, Heading{Level: v.Level, ID: string(id.([]byte)), Text: textOf(v, body), when: when})
				}
			}
		case *Figure:
			d.IDs[fmt.Sprintf("figure-%d", v.Index)] = true
		case *ast.Image:
			d.Images = append(d.Images, string(v.Destination))
			d.Pictures = append(d.Pictures, Image{Dest: string(v.Destination), Alt: textOf(v, body)})
		case *ast.Link:
			d.Links = append(d.Links, string(v.Destination))
		case *ast.HTMLBlock, *ast.RawHTML:
			var raw []byte
			if b, ok := v.(*ast.HTMLBlock); ok {
				for i := 0; i < b.Lines().Len(); i++ {
					seg := b.Lines().At(i)
					raw = append(raw, seg.Value(body)...)
				}
			} else {
				r := v.(*ast.RawHTML)
				for i := 0; i < r.Segments.Len(); i++ {
					seg := r.Segments.At(i)
					raw = append(raw, seg.Value(body)...)
				}
			}
			for _, m := range rawID.FindAllSubmatch(raw, -1) {
				d.IDs[string(m[1])] = true
			}
		}
		return ast.WalkContinue, nil
	})
}

// HeadingsFor lists the level 2 and 3 headings that format shows.
func (d *Document) HeadingsFor(format string) []Heading {
	var out []Heading
	for _, h := range d.Headings {
		if h.when.visible(format) {
			out = append(out, h)
		}
	}
	return out
}

// Render writes the chapter body as an HTML fragment for one format.
func (d *Document) Render(w io.Writer, opts RenderOptions) error {
	if opts.Format == "" {
		opts.Format = "html"
	}
	d.prepare(opts)
	r := md.Renderer()
	for _, b := range d.blocks {
		if !b.when.visible(opts.Format) {
			continue
		}
		if err := r.Render(w, d.source, b.node); err != nil {
			return err
		}
	}
	return nil
}

// PlainText returns the words the website shows, without markup, for search.
func (d *Document) PlainText() string {
	if d.plain == nil {
		var parts []string
		for _, b := range d.blocks {
			if b.when.visible("html") {
				parts = append(parts, textOf(b.node, d.source))
			}
		}
		text := strings.Join(parts, " ")
		d.plain = &text
	}
	return *d.plain
}

// RewriteLinks passes every link destination through fn.
func (d *Document) RewriteLinks(fn func(dest string) string) {
	_ = ast.Walk(d.root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if l, ok := n.(*ast.Link); ok && entering {
			l.Destination = []byte(fn(string(l.Destination)))
		}
		return ast.WalkContinue, nil
	})
}

func splitFrontMatter(src []byte) (front, body []byte) {
	if !bytes.HasPrefix(src, []byte("---\n")) && !bytes.HasPrefix(src, []byte("---\r\n")) {
		return nil, src
	}
	rest := src[bytes.IndexByte(src, '\n')+1:]
	for off := 0; off < len(rest); {
		line, next := rest[off:], len(rest)
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line, next = line[:i], off+i+1
		}
		if string(bytes.TrimRight(line, " \t\r")) == "---" {
			return rest[:off], rest[next:]
		}
		off = next
	}
	return nil, src
}

func textOf(n ast.Node, src []byte) string {
	var b strings.Builder
	writeText(n, src, &b)
	return strings.Join(strings.Fields(b.String()), " ")
}

func writeText(n ast.Node, src []byte, b *strings.Builder) {
	switch v := n.(type) {
	case *ast.Text:
		b.Write(v.Segment.Value(src))
		if v.SoftLineBreak() || v.HardLineBreak() {
			b.WriteByte(' ')
		}
		return
	case *ast.String:
		b.WriteString(html.UnescapeString(string(v.Value))) // typographer output such as &ldquo;
		return
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		lines := n.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			b.Write(seg.Value(src))
		}
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		writeText(c, src, b)
	}
	if n.Type() == ast.TypeBlock {
		b.WriteByte(' ')
	}
}
