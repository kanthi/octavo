package markdown

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Callout is a block quote that starts with a marker line such as "[!NOTE]".
type Callout struct {
	ast.BaseBlock
	Name string // note, tip, warning, important or caution
}

var KindCallout = ast.NewNodeKind("Callout")

func (n *Callout) Kind() ast.NodeKind { return KindCallout }

func (n *Callout) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Name": n.Name}, nil)
}

var calloutMarker = regexp.MustCompile(`^\[!(NOTE|TIP|WARNING|IMPORTANT|CAUTION)\]$`)

// calloutTransformer replaces marked block quotes with Callout nodes.
type calloutTransformer struct{}

func (calloutTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	var quotes []*ast.Blockquote
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if q, ok := n.(*ast.Blockquote); ok && entering {
			quotes = append(quotes, q)
		}
		return ast.WalkContinue, nil
	})
	for _, q := range quotes {
		p, ok := q.FirstChild().(*ast.Paragraph)
		if !ok || p.Lines().Len() == 0 {
			continue
		}
		first := p.Lines().At(0)
		m := calloutMarker.FindSubmatch(bytes.TrimSpace(first.Value(src)))
		if m == nil {
			continue
		}
		// Drop the marker line's text, keep the rest of the paragraph.
		for c := p.FirstChild(); c != nil; {
			t, ok := c.(*ast.Text)
			if !ok || t.Segment.Start < first.Start || t.Segment.Stop > first.Stop {
				break
			}
			next := c.NextSibling()
			p.RemoveChild(p, c)
			c = next
		}
		if p.ChildCount() == 0 {
			q.RemoveChild(q, p)
		}
		callout := &Callout{Name: strings.ToLower(string(m[1]))}
		for c := q.FirstChild(); c != nil; {
			next := c.NextSibling()
			callout.AppendChild(callout, c)
			c = next
		}
		q.Parent().ReplaceChild(q.Parent(), q, callout)
	}
}

type calloutRenderer struct{}

func (calloutRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindCallout, func(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			_, _ = w.WriteString("</div>\n")
			return ast.WalkContinue, nil
		}
		kind := n.(*Callout).Name
		title := strings.ToUpper(kind[:1]) + kind[1:]
		_, _ = w.WriteString(`<div class="callout callout-` + kind + `" role="note">` + "\n" +
			`<p class="callout-title">` + title + "</p>\n")
		return ast.WalkContinue, nil
	})
}
