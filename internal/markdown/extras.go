package markdown

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// RenderOptions say which output a chapter is rendered for.
type RenderOptions struct {
	// Format is "html", "epub" or "pdf". Content marked for other formats is
	// left out (see the format blocks in package book).
	Format string
	// FigurePrefix goes before each figure's number: "3." makes "Figure 3.1".
	FigurePrefix string
	// Dark maps an image's destination to its dark-page version. Only the
	// website uses it.
	Dark map[string]string
	// Lazy lets the browser load images only when they scroll into view. Not
	// for PDF: the browser prints before it would load them.
	Lazy bool
}

// condition is the stack of format blocks around a part of a chapter.
type condition []rule

type rule struct {
	show    bool // content-visible (true) or content-hidden (false)
	formats []string
}

func (c condition) visible(format string) bool {
	for _, r := range c {
		listed := false
		for _, f := range r.formats {
			listed = listed || f == format
		}
		if listed != r.show {
			return false
		}
	}
	return true
}

// Format blocks reach the parser as these comments; package book writes them.
var (
	whenComment = regexp.MustCompile(`^<!-- octavo:when (visible|hidden) ([a-z,]+) -->\s*$`)
	endComment  = regexp.MustCompile(`^<!-- octavo:end -->\s*$`)
)

// WhenComment and EndComment are the markers package book writes around a
// block of content that only some formats show.
func WhenComment(show bool, formats []string) string {
	mode := "hidden"
	if show {
		mode = "visible"
	}
	return "<!-- octavo:when " + mode + " " + strings.Join(formats, ",") + " -->"
}

const EndComment = "<!-- octavo:end -->"

// marker reports whether n is a format-block comment, and which.
func marker(n ast.Node, src []byte) (open *rule, end bool) {
	b, ok := n.(*ast.HTMLBlock)
	if !ok || b.Lines().Len() != 1 {
		return nil, false
	}
	line := b.Lines().At(0)
	s := string(line.Value(src))
	if m := whenComment.FindStringSubmatch(s); m != nil {
		return &rule{show: m[1] == "visible", formats: strings.Split(m[2], ",")}, false
	}
	return nil, endComment.MatchString(s)
}

// Figure is an image that stands alone in its paragraph: it is numbered and
// gets a caption.
type Figure struct {
	ast.BaseBlock
	Index   int    // 1, 2, 3 within the chapter
	Caption string // the image's title, else its alt text
	label   string // "Figure 3.1", set before each render
}

var KindFigure = ast.NewNodeKind("Figure")

func (f *Figure) Kind() ast.NodeKind { return KindFigure }
func (f *Figure) Dump(src []byte, level int) {
	ast.DumpHelper(f, src, level, map[string]string{"Index": strconv.Itoa(f.Index)}, nil)
}

// imageAttrs matches "{width=60%}" right after an image.
var (
	imageAttrs = regexp.MustCompile(`^\{([^{}\n]*)\}`)
	sizeValue  = regexp.MustCompile(`^\d+(\.\d+)?(%|px|em|rem|cm|mm|in|pt)?$`)
)

// figureTransformer reads image sizes and turns lone images into figures.
type figureTransformer struct{}

func (figureTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	src := reader.Source()
	var images []*ast.Image
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if img, ok := n.(*ast.Image); ok && entering {
			images = append(images, img)
		}
		return ast.WalkContinue, nil
	})
	for _, img := range images {
		applySize(img, src)
	}
	index := 0
	for _, img := range images {
		p, ok := img.Parent().(*ast.Paragraph)
		if !ok || !alone(img, p, src) {
			continue
		}
		caption := string(img.Title)
		if caption == "" {
			caption = textOf(img, src)
		}
		if caption == "" {
			continue // no words to caption it with: an ordinary image
		}
		index++
		fig := &Figure{Index: index, Caption: caption}
		p.Parent().ReplaceChild(p.Parent(), p, fig)
		p.RemoveChild(p, img)
		fig.AppendChild(fig, img)
	}
}

// applySize reads "{width=60% height=10em}" after an image into its style.
func applySize(img *ast.Image, src []byte) {
	t, ok := img.NextSibling().(*ast.Text)
	if !ok {
		return
	}
	value := t.Segment.Value(src)
	m := imageAttrs.FindSubmatch(value)
	if m == nil {
		return
	}
	var style []string
	for _, pair := range strings.Fields(string(m[1])) {
		key, val, ok := strings.Cut(pair, "=")
		val = strings.Trim(val, `"'`)
		if !ok || (key != "width" && key != "height") || !sizeValue.MatchString(val) {
			return // not a size we understand: leave the text as written
		}
		if _, err := strconv.ParseFloat(val, 64); err == nil {
			val += "px"
		}
		style = append(style, key+": "+val)
	}
	img.SetAttributeString("style", []byte(strings.Join(style, "; ")))
	if len(m[0]) == len(value) && !t.SoftLineBreak() && !t.HardLineBreak() {
		t.Parent().RemoveChild(t.Parent(), t)
		return
	}
	t.Segment = text.NewSegment(t.Segment.Start+len(m[0]), t.Segment.Stop)
}

// alone reports whether img is the only thing in its paragraph.
func alone(img *ast.Image, p *ast.Paragraph, src []byte) bool {
	for c := p.FirstChild(); c != nil; c = c.NextSibling() {
		if c == img {
			continue
		}
		if t, ok := c.(*ast.Text); ok && strings.TrimSpace(string(t.Segment.Value(src))) == "" {
			continue
		}
		return false
	}
	return true
}

// extraRenderer writes figures and images.
type extraRenderer struct{}

func (extraRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(KindFigure, renderFigure)
	r.Register(ast.KindImage, renderImage)
}

func renderFigure(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	f := n.(*Figure)
	if entering {
		fmt.Fprintf(w, "<figure id=\"figure-%d\">\n", f.Index)
		return ast.WalkContinue, nil
	}
	fmt.Fprintf(w, "\n<figcaption><span class=\"figure-number\">%s</span> %s</figcaption>\n</figure>\n",
		html.EscapeString(f.label), html.EscapeString(f.Caption))
	return ast.WalkContinue, nil
}

// renderImage writes an <img>, or two when the image has a version for dark
// pages: the page's stylesheet shows one or the other.
func renderImage(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	img := n.(*ast.Image)
	alt := textOf(img, src)
	one := func(dest []byte, class string) {
		w.WriteString(`<img src="`)
		w.Write(util.EscapeHTML(util.URLEscape(dest, true)))
		w.WriteString(`" alt="`)
		w.WriteString(html.EscapeString(alt))
		w.WriteString(`"`)
		if len(img.Title) > 0 {
			w.WriteString(` title="` + html.EscapeString(string(img.Title)) + `"`)
		}
		if class != "" {
			w.WriteString(` class="` + class + `"`)
		}
		if v, ok := img.AttributeString("style"); ok {
			w.WriteString(` style="` + html.EscapeString(string(v.([]byte))) + `"`)
		}
		if _, ok := img.AttributeString("data-octavo-lazy"); ok {
			w.WriteString(` loading="lazy"`)
		}
		w.WriteString(">")
	}
	if dark, ok := img.AttributeString("data-octavo-dark"); ok {
		one(img.Destination, "light-only")
		one(dark.([]byte), "dark-only")
	} else {
		one(img.Destination, "")
	}
	return ast.WalkSkipChildren, nil
}

// prepare sets what a render depends on: figure labels and image attributes.
func (d *Document) prepare(opts RenderOptions) {
	_ = ast.Walk(d.root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *Figure:
			v.label = "Figure " + opts.FigurePrefix + strconv.Itoa(v.Index)
		case *ast.Image:
			v.SetAttributeString("data-octavo-dark", nil)
			v.SetAttributeString("data-octavo-lazy", nil)
			removeNilAttrs(v)
			if dark, ok := opts.Dark[string(v.Destination)]; ok {
				v.SetAttributeString("data-octavo-dark", []byte(dark))
			}
			if opts.Lazy {
				v.SetAttributeString("data-octavo-lazy", []byte("1"))
			}
		}
		return ast.WalkContinue, nil
	})
}

// removeNilAttrs drops attributes set to nil, so that "has attribute" means
// "set for this render".
func removeNilAttrs(n ast.Node) {
	attrs := n.Attributes()
	n.RemoveAttributes()
	for _, a := range attrs {
		if a.Value != nil {
			if b, ok := a.Value.([]byte); !ok || b != nil {
				n.SetAttribute(a.Name, a.Value)
			}
		}
	}
}
