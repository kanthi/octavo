package pdf

// The Typst engine: the book is written as one Typst file and compiled by
// the typst program. Chapters are rendered to HTML as for the other formats,
// and that HTML is turned into Typst markup here, so callouts, figures, task
// lists and raw HTML come out the same as on the website.

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/format"
	"github.com/kanthi/octavo/internal/format/fonts"
	"github.com/kanthi/octavo/internal/format/xhtml"
	"github.com/kanthi/octavo/internal/markdown"
)

//go:embed book.typ
var typstPreamble string

func (f *Format) renderTypst(b *book.Book, out *format.Output, opts format.Options, typst string) error {
	name := f.File(b)
	tmp, err := os.MkdirTemp("", "octavo-typst-")
	if err != nil {
		return err
	}
	// OCTAVO_TYPST_KEEP=1 keeps the Typst source, to look into a problem.
	keep := os.Getenv("OCTAVO_TYPST_KEEP") != ""
	if keep {
		fmt.Fprintf(opts.Log, "%s: Typst source kept in %s\n", name, tmp)
	}
	defer func() {
		if !keep {
			os.RemoveAll(tmp)
		}
	}()
	src, err := f.typstDocument(b, opts, tmp)
	if err != nil {
		return err
	}
	fingerprint := imagesFingerprint(b, format.Hash(src, engineTypst))
	if out.Fresh(name, fingerprint) {
		return nil
	}
	fontDir := filepath.Join(tmp, "fonts")
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		return err
	}
	which := []fonts.Font{fonts.Mono}
	if font, ok := fonts.Find(f.s.Font); ok {
		which = append(which, font)
	}
	if err := fonts.WriteTTF(fontDir, which...); err != nil {
		return err
	}
	input, output := filepath.Join(tmp, "book.typ"), filepath.Join(tmp, "book.pdf")
	if err := os.WriteFile(input, src, 0o644); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(f.s.Timeout)*time.Second)
	defer cancel()
	// The root is the whole disk, so that the book's images can be named by
	// their full path.
	root := filepath.VolumeName(tmp) + string(filepath.Separator)
	cmd := exec.CommandContext(ctx, typst, "compile", "--root", root, "--font-path", fontDir, input, output)
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Run(); err != nil {
		keep = true
		if ctx.Err() != nil {
			return fmt.Errorf("typst took longer than %d seconds (raise \"timeout\" under format: pdf:)", f.s.Timeout)
		}
		return fmt.Errorf("typst: %w\n%s\n(the Typst source is kept in %s)", err, lastLines(log.String(), 20), input)
	}
	if w := strings.TrimSpace(log.String()); w != "" {
		fmt.Fprintf(opts.Log, "warning: %s: typst says:\n%s\n", name, lastLines(w, 12))
	}
	data, err := os.ReadFile(output)
	if err != nil || len(data) == 0 {
		return fmt.Errorf("typst finished but wrote no file")
	}
	return out.Write(name, fingerprint, data)
}

// typstDocument writes the whole book as Typst markup. Inline SVG images are
// written into dir, beside the book.typ file.
func (f *Format) typstDocument(b *book.Book, opts format.Options, dir string) ([]byte, error) {
	info := b.Config.Book
	numbered := on(f.s.NumberChapters)
	anchor := func(i int) string { return fmt.Sprintf("ch%d", i+1) }
	index := map[*book.Chapter]int{}
	for i, ch := range b.Chapters {
		index[ch] = i
	}

	// Every chapter's HTML first, so that links can be checked against every
	// label in the book: Typst stops on a link to a label that is not there.
	c := &typConv{
		columns:  f.codeColumns(),
		resolver: xhtml.NewResolver(b),
		labels:   map[string]bool{},
		dir:      dir,
		log:      opts.Log,
		book:     b,
		anchor:   anchor,
	}
	trees := make([][]*nethtml.Node, len(b.Chapters))
	for i, ch := range b.Chapters {
		var src bytes.Buffer
		if err := ch.Doc.Render(&src, markdown.RenderOptions{Format: "pdf", FigurePrefix: xhtml.FigurePrefix(ch, numbered)}); err != nil {
			return nil, fmt.Errorf("chapter %s: %w", ch.Path, err)
		}
		nodes, err := nethtml.ParseFragment(&src, &nethtml.Node{Type: nethtml.ElementNode, Data: "body", DataAtom: atom.Body})
		if err != nil {
			return nil, fmt.Errorf("chapter %s: %w", ch.Path, err)
		}
		trees[i] = nodes
		c.labels[anchor(i)] = true
		for _, n := range nodes {
			c.collectIDs(n, anchor(i)+"-")
		}
	}

	var s strings.Builder
	s.WriteString(f.typstSettings(b))
	s.WriteString(typstPreamble)

	// Title page.
	s.WriteString("\n#title-page(" + typStr(info.Title) + ", " + typStr(info.Author) + ", " + typStr(info.Date) + ", " + typStr(info.Description) + ")\n")

	// Contents, with page numbers.
	if on(f.s.TOC) {
		s.WriteString("\n#contents-heading()\n")
		var toc func(nodes []*book.Node, depth int)
		toc = func(nodes []*book.Node, depth int) {
			for _, n := range nodes {
				switch {
				case n.Chapter != nil:
					num := "none"
					if numbered && n.Chapter.Number > 0 {
						num = typStr(strconv.Itoa(n.Chapter.Number))
					}
					fmt.Fprintf(&s, "#toc-line(%s, [%s], <%s>, %d)\n", num, typText(n.Title), anchor(index[n.Chapter]), depth)
				case len(n.Children) > 0:
					fmt.Fprintf(&s, "#toc-group([%s], %d)\n", typText(n.Title), depth)
					toc(n.Children, depth+1)
				}
			}
		}
		toc(b.Nav, 0)
	}

	// The body is numbered 1, 2, 3 from its first page.
	s.WriteString("\n#pagebreak(weak: true)\n#set page(numbering: \"1\")\n#counter(page).update(1)\n")

	parts := 0
	var walk func(nodes []*book.Node, depth int)
	walk = func(nodes []*book.Node, depth int) {
		for _, n := range nodes {
			if n.Chapter == nil {
				if depth == 0 && len(n.Children) > 0 {
					parts++
					fmt.Fprintf(&s, "\n#part-page(%d, [%s])\n", parts, typText(n.Title))
				}
				walk(n.Children, depth+1)
				continue
			}
			ch, i := n.Chapter, index[n.Chapter]
			kicker := "none"
			if numbered && ch.Number > 0 {
				kicker = typStr(fmt.Sprintf("Chapter %d", ch.Number))
			}
			left := info.Title
			if ch.Title == info.Title { // the preface is often named after the book; say it once
				left = ""
			}
			fmt.Fprintf(&s, "\n#chapter-start(%s, %s, %s)\n#heading(level: 1)[%s] <%s>\n\n", kicker, typStr(left), typStr(xhtml.Label(ch, numbered)), typText(ch.Title), anchor(i))
			s.WriteString(c.chapter(ch, anchor(i)+"-", trees[i]))
			s.WriteString("\n")
		}
	}
	walk(b.Nav, 0)
	return []byte(s.String()), nil
}

// typstSettings sets what octavo.yml decides: paper, margins, font, size.
func (f *Format) typstSettings(b *book.Book) string {
	family := `("Charter", "Iowan Old Style", "Georgia", "Libertinus Serif")`
	if font, ok := fonts.Find(f.s.Font); ok {
		family = "(" + typStr(font.Family) + ", \"Libertinus Serif\")"
	}
	paper := "paper: \"a4\""
	switch p := strings.ToLower(strings.TrimSpace(f.s.Paper)); p {
	case "a3", "a4", "a5", "a6":
		paper = "paper: \"" + p + "\""
	case "b5":
		paper = "paper: \"iso-b5\""
	case "letter":
		paper = "paper: \"us-letter\""
	case "legal":
		paper = "paper: \"us-legal\""
	default:
		if parts := strings.Fields(p); len(parts) == 2 {
			paper = "width: " + typLength(parts[0]) + ", height: " + typLength(parts[1])
		}
	}
	m := strings.Fields(f.s.Margin)
	for i := range m {
		m[i] = typLength(m[i])
	}
	var top, right, bottom, left string
	switch len(m) {
	case 1:
		top, right, bottom, left = m[0], m[0], m[0], m[0]
	case 2:
		top, right, bottom, left = m[0], m[1], m[0], m[1]
	case 3:
		top, right, bottom, left = m[0], m[1], m[2], m[1]
	case 4:
		top, right, bottom, left = m[0], m[1], m[2], m[3]
	default:
		top, right, bottom, left = "26mm", "25mm", "28mm", "25mm"
	}
	lang := b.Config.Book.Lang
	if lang == "" {
		lang = "en"
	}
	if i := strings.IndexAny(lang, "-_"); i > 0 {
		lang = lang[:i]
	}
	return fmt.Sprintf(`// Made by Octavo from %s.
#let octavo = (
  title: %s,
  author: %s,
  paper: (%s),
  margin: (top: %s, right: %s, bottom: %s, left: %s),
  font: %s,
  size: %s,
  lang: %s,
  justify: %t,
  headers: %t,
  page-numbers: %t,
)
`, typStr(filepath.Base(b.Dir)), typStr(b.Config.Book.Title), typStr(b.Config.Book.Author), paper, top, right, bottom, left,
		family, typLength(f.s.FontSize), typStr(strings.ToLower(lang)), on(f.s.Justify), on(f.s.Headers), on(f.s.PageNumbers))
}

// breakLong lets a line break inside runs of code with no spaces that are
// longer than limit characters, the way the browser engine's
// "overflow-wrap: anywhere" does. It adds zero-width spaces after
// punctuation, or every 16 characters where there is none.
func breakLong(text string, limit int) string {
	if limit < 8 {
		return text
	}
	var out strings.Builder
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			out.WriteByte('\n')
		}
		if utf8.RuneCountInString(line) <= limit {
			out.WriteString(line)
			continue
		}
		run := 0
		for _, r := range line {
			out.WriteRune(r)
			if r == ' ' || r == '\t' {
				run = 0
				continue
			}
			run++
			if run >= limit/2 && strings.ContainsRune(".,;:/()=_-&?", r) || run >= 16 && run%16 == 0 && run > limit/2 {
				out.WriteRune('\u200b')
				if run >= limit/2 {
					run = limit / 2 // keep breaking every so often
				}
			}
		}
	}
	return out.String()
}

// codeColumns is how many characters of code fit across a code block on
// the page, from the paper, margins and font size (code is set at 0.8 of
// the text, in a font whose letters are 0.6 em wide).
func (f *Format) codeColumns() int {
	widths := map[string]float64{"a3": 841.89, "a4": 595.28, "a5": 419.53, "a6": 297.64, "b5": 498.9, "letter": 612, "legal": 612}
	paper, ok := widths[strings.ToLower(strings.TrimSpace(f.s.Paper))]
	if !ok {
		if parts := strings.Fields(f.s.Paper); len(parts) == 2 {
			paper = points(parts[0])
		}
	}
	m := strings.Fields(f.s.Margin)
	var left, right float64
	switch len(m) {
	case 1:
		left, right = points(m[0]), points(m[0])
	case 2, 3:
		left, right = points(m[1]), points(m[1])
	case 4:
		left, right = points(m[3]), points(m[1])
	}
	size := points(f.s.FontSize)
	if paper == 0 || size == 0 {
		return 0
	}
	usable := paper - left - right - 2*0.9*size - 2 // the block's padding and border
	return int(usable / (0.602 * 0.8 * size))
}

// points reads a CSS length in points; 0 when it cannot.
func points(v string) float64 {
	m := cssLength.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return 0
	}
	n, _ := strconv.ParseFloat(m[1], 64)
	return n * map[string]float64{"mm": 72 / 25.4, "cm": 72 / 2.54, "in": 72, "pt": 1, "px": 0.75, "": 1, "em": 0}[m[2]]
}

var cssLength = regexp.MustCompile(`^([0-9]*\.?[0-9]+)(mm|cm|in|pt|px|em)?$`)

// typLength turns a CSS length into a Typst one: px become pt.
func typLength(v string) string {
	m := cssLength.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return "1em"
	}
	switch m[2] {
	case "px":
		n, _ := strconv.ParseFloat(m[1], 64)
		return strconv.FormatFloat(n*0.75, 'f', -1, 64) + "pt"
	case "":
		if m[1] == "0" {
			return "0pt"
		}
		return m[1] + "pt"
	}
	return m[1] + m[2]
}

// typStr writes text as a Typst string literal.
func typStr(text string) string {
	var s strings.Builder
	s.WriteByte('"')
	for _, r := range text {
		switch r {
		case '\\':
			s.WriteString(`\\`)
		case '"':
			s.WriteString(`\"`)
		case '\n':
			s.WriteString(`\n`)
		case '\t':
			s.WriteString(`\t`)
		case '\r':
		default:
			if r < 0x20 {
				continue
			}
			s.WriteRune(r)
		}
	}
	s.WriteByte('"')
	return s.String()
}

// typText writes text as Typst markup that shows exactly that text.
func typText(text string) string {
	var s strings.Builder
	prev := ' '
	for _, r := range text {
		switch {
		case strings.ContainsRune("\\#*_`$<>@[]~=-+/\"':", r):
			s.WriteByte('\\')
			s.WriteRune(r)
		case r == '.' && unicode.IsDigit(prev): // "1." at the start of a line is a list
			s.WriteString(`\.`)
		case r < 0x20:
			s.WriteByte(' ')
		default:
			s.WriteRune(r)
		}
		prev = r
	}
	return s.String()
}

// typLabel makes an id usable as a Typst label.
func typLabel(id string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' || r == ':' {
			return r
		}
		return '_'
	}, id)
}

var spaces = regexp.MustCompile(`[ \t\r\n]+`)

// typConv turns a chapter's HTML into Typst markup.
type typConv struct {
	columns  int // characters of code that fit across the page
	resolver *xhtml.Resolver
	labels   map[string]bool // every label in the book
	placed   map[string]bool // labels already placed
	dir      string
	svgs     int
	log      interface{ Write([]byte) (int, error) }
	book     *book.Book
	anchor   func(int) string

	// Per chapter.
	ch        *book.Chapter
	prefix    string
	footnotes map[string]*nethtml.Node // "fn:1" -> its <li>
	noted     map[string]string        // footnotes already placed -> their label
}

func (c *typConv) warn(format string, args ...any) {
	fmt.Fprintf(c.log, "warning: %s: "+format+"\n", append([]any{c.ch.Path}, args...)...)
}

func (c *typConv) collectIDs(n *nethtml.Node, prefix string) {
	if n.Type != nethtml.ElementNode {
		return
	}
	if xhtml.HasClass(n, "footnotes") {
		return
	}
	if id := xhtml.Attr(n, "id"); id != "" {
		c.labels[typLabel(prefix+id)] = true
	}
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		c.collectIDs(k, prefix)
	}
}

func (c *typConv) chapter(ch *book.Chapter, prefix string, nodes []*nethtml.Node) string {
	if c.placed == nil {
		c.placed = map[string]bool{}
	}
	c.ch, c.prefix = ch, prefix
	c.footnotes, c.noted = map[string]*nethtml.Node{}, map[string]string{}
	root := &nethtml.Node{Type: nethtml.ElementNode, Data: "body", DataAtom: atom.Body}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	var find func(n *nethtml.Node)
	find = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.Data == "li" {
			if id := xhtml.Attr(n, "id"); strings.HasPrefix(id, "fn:") {
				c.footnotes[id] = n
			}
		}
		for k := n.FirstChild; k != nil; k = k.NextSibling {
			find(k)
		}
	}
	find(root)
	return c.blocks(root)
}

// label places the label for an element's id, once.
func (c *typConv) label(n *nethtml.Node) string {
	id := xhtml.Attr(n, "id")
	if id == "" {
		return ""
	}
	l := typLabel(c.prefix + id)
	if c.placed[l] {
		return ""
	}
	c.placed[l] = true
	return "<" + l + ">"
}

var blockTags = map[string]bool{
	"p": true, "div": true, "pre": true, "ul": true, "ol": true, "dl": true, "table": true, "blockquote": true,
	"figure": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "hr": true,
	"section": true, "article": true, "aside": true, "header": true, "footer": true, "nav": true, "main": true,
	"details": true, "summary": true, "center": true, "figcaption": true, "address": true, "fieldset": true, "form": true,
}

func isBlock(n *nethtml.Node) bool {
	return n.Type == nethtml.ElementNode && n.Namespace == "" && blockTags[n.Data]
}

// blocks writes the children of n as a run of blocks.
func (c *typConv) blocks(n *nethtml.Node) string {
	var out []string
	var run strings.Builder
	flush := func() {
		if t := strings.TrimSpace(run.String()); t != "" {
			out = append(out, t)
		}
		run.Reset()
	}
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		if isBlock(k) {
			flush()
			if s := strings.TrimSpace(c.block(k)); s != "" {
				out = append(out, s)
			}
		} else if k.Type == nethtml.ElementNode && k.Namespace == "svg" && k.Data == "svg" {
			flush()
			out = append(out, c.svg(k, false))
		} else {
			run.WriteString(c.inline(k))
		}
	}
	flush()
	return strings.Join(out, "\n\n")
}

func (c *typConv) inlines(n *nethtml.Node) string {
	var s strings.Builder
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		s.WriteString(c.inline(k))
	}
	return s.String()
}

func (c *typConv) block(n *nethtml.Node) string {
	mark := ""
	if !strings.HasPrefix(n.Data, "h") || len(n.Data) != 2 {
		if l := c.label(n); l != "" {
			mark = "#metadata(none) " + l + "\n"
		}
	}
	switch n.Data {
	case "p":
		if img := loneImage(n); img != nil {
			return mark + c.image(img, true)
		}
		return mark + c.inlines(n)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(n.Data[1]-'0') + 0
		if level < 2 {
			level = 2
		}
		if level > 5 {
			level = 5
		}
		h := fmt.Sprintf("#heading(level: %d)[%s]", level, strings.TrimSpace(c.inlines(n)))
		if l := c.label(n); l != "" {
			h += " " + l
		}
		return h
	case "pre":
		return mark + c.code(n)
	case "ul", "ol":
		return mark + c.list(n)
	case "dl":
		var s strings.Builder
		for k := n.FirstChild; k != nil; k = k.NextSibling {
			switch k.Data {
			case "dt":
				s.WriteString("#strong[" + strings.TrimSpace(c.inlines(k)) + "]\n\n")
			case "dd":
				s.WriteString("#pad(left: 1.5em)[" + c.blocks(k) + "]\n\n")
			}
		}
		return mark + s.String()
	case "table":
		return mark + c.table(n)
	case "blockquote":
		return mark + "#octavo-quote[" + c.blocks(n) + "]"
	case "figure":
		return mark + c.figure(n)
	case "hr":
		return "#octavo-rule()"
	case "details":
		return mark + c.blocks(n)
	case "summary":
		return "#strong[" + strings.TrimSpace(c.inlines(n)) + "]"
	case "figcaption":
		return mark + "#octavo-caption[" + c.inlines(n) + "]"
	case "div":
		if xhtml.HasClass(n, "footnotes") {
			return ""
		}
		if xhtml.HasClass(n, "callout") {
			return mark + c.callout(n)
		}
	}
	return mark + c.blocks(n)
}

func loneImage(p *nethtml.Node) *nethtml.Node {
	var img *nethtml.Node
	for k := p.FirstChild; k != nil; k = k.NextSibling {
		switch {
		case k.Type == nethtml.TextNode && strings.TrimSpace(k.Data) == "":
		case k.Type == nethtml.ElementNode && k.Data == "img" && img == nil:
			img = k
		default:
			return nil
		}
	}
	return img
}

func (c *typConv) callout(n *nethtml.Node) string {
	kind := "note"
	for _, k := range []string{"note", "tip", "important", "warning", "caution"} {
		if xhtml.HasClass(n, "callout-"+k) {
			kind = k
		}
	}
	title := ""
	body := &nethtml.Node{Type: nethtml.ElementNode, Data: "div"}
	for k := n.FirstChild; k != nil; {
		next := k.NextSibling
		if k.Type == nethtml.ElementNode && xhtml.HasClass(k, "callout-title") && title == "" {
			title = strings.TrimSpace(c.inlines(k))
		} else {
			n.RemoveChild(k)
			body.AppendChild(k)
		}
		k = next
	}
	return fmt.Sprintf("#callout(%s, [%s])[%s]", typStr(kind), title, c.blocks(body))
}

func (c *typConv) code(pre *nethtml.Node) string {
	lang := xhtml.Attr(pre, "data-lang")
	if lang == "" {
		for _, el := range []*nethtml.Node{pre, pre.FirstChild} {
			if el == nil || el.Type != nethtml.ElementNode {
				continue
			}
			for _, cls := range strings.Fields(xhtml.Attr(el, "class")) {
				if l, ok := strings.CutPrefix(cls, "language-"); ok {
					lang = l
				} else if cls != "sourceCode" && cls != "chroma" && lang == "" && el == pre {
					lang = cls
				}
			}
		}
	}
	text := strings.TrimRight(textOf(pre), "\n")
	if c.columns > 0 {
		// Even at 70%, a line this long would run off the page.
		text = breakLong(text, int(float64(c.columns)/0.7))
	}
	args := "block: true"
	if lang != "" {
		args += ", lang: " + typStr(strings.ToLower(lang))
	}
	code := "#raw(" + args + ", " + typStr(text) + ")"
	// A block with lines too long for the page is set smaller, down to 70%,
	// so that its lines stay whole. Longer lines than that still wrap at
	// spaces, like the browser's.
	widest := 0
	for _, line := range strings.Split(text, "\n") {
		widest = max(widest, utf8.RuneCountInString(strings.ReplaceAll(line, "\t", "    ")))
	}
	if c.columns > 0 && widest > c.columns {
		scale := max(0.7, float64(c.columns)/float64(widest))
		return fmt.Sprintf("#text(size: %.3fem)[%s]", scale, code)
	}
	return code
}

func textOf(n *nethtml.Node) string {
	if n.Type == nethtml.TextNode {
		return n.Data
	}
	var s strings.Builder
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		s.WriteString(textOf(k))
	}
	return s.String()
}

func (c *typConv) list(n *nethtml.Node) string {
	var items []string
	tight := true
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		if k.Type != nethtml.ElementNode || k.Data != "li" {
			continue
		}
		for p := k.FirstChild; p != nil; p = p.NextSibling {
			if p.Type == nethtml.ElementNode && p.Data == "p" {
				tight = false
			}
		}
		items = append(items, "["+c.blocks(k)+"]")
	}
	if len(items) == 0 {
		return ""
	}
	fn := "list"
	args := fmt.Sprintf("tight: %t", tight)
	if n.Data == "ol" {
		fn = "enum"
		if start := xhtml.Attr(n, "start"); start != "" {
			if v, err := strconv.Atoi(start); err == nil && v >= 0 {
				args += fmt.Sprintf(", start: %d", v)
			}
		}
	}
	return "#" + fn + "(" + args + ",\n" + strings.Join(items, ",\n") + ")"
}

func (c *typConv) table(n *nethtml.Node) string {
	type row struct {
		head  bool
		cells []*nethtml.Node
	}
	var rows []row
	var walk func(el *nethtml.Node, head bool)
	walk = func(el *nethtml.Node, head bool) {
		for k := el.FirstChild; k != nil; k = k.NextSibling {
			if k.Type != nethtml.ElementNode {
				continue
			}
			switch k.Data {
			case "thead":
				walk(k, true)
			case "tbody", "tfoot":
				walk(k, false)
			case "tr":
				r := row{head: head}
				for td := k.FirstChild; td != nil; td = td.NextSibling {
					if td.Type == nethtml.ElementNode && (td.Data == "td" || td.Data == "th") {
						r.cells = append(r.cells, td)
					}
				}
				if len(r.cells) > 0 {
					rows = append(rows, r)
				}
			}
		}
	}
	walk(n, false)
	if len(rows) == 0 {
		return ""
	}
	span := func(el *nethtml.Node, key string) int {
		v, err := strconv.Atoi(xhtml.Attr(el, key))
		if err != nil || v < 1 {
			return 1
		}
		return v
	}
	columns := 0
	for _, r := range rows {
		w := 0
		for _, td := range r.cells {
			w += span(td, "colspan")
		}
		columns = max(columns, w)
	}
	// Column alignment, from the first row's cells.
	var aligns []string
	for _, td := range rows[0].cells {
		a := "auto"
		style := strings.ReplaceAll(xhtml.Attr(td, "style")+";align:"+xhtml.Attr(td, "align"), " ", "")
		switch {
		case strings.Contains(style, "align:center"):
			a = "center"
		case strings.Contains(style, "align:right"):
			a = "right"
		case strings.Contains(style, "align:left"):
			a = "left"
		}
		for i := 0; i < span(td, "colspan"); i++ {
			aligns = append(aligns, a)
		}
	}
	for len(aligns) < columns {
		aligns = append(aligns, "auto")
	}
	cell := func(td *nethtml.Node) string {
		body := c.blocks(td)
		cs, rs := span(td, "colspan"), span(td, "rowspan")
		if cs == 1 && rs == 1 {
			return "[" + body + "]"
		}
		return fmt.Sprintf("table.cell(colspan: %d, rowspan: %d)[%s]", cs, rs, body)
	}
	var s strings.Builder
	fmt.Fprintf(&s, "#octavo-table(columns: %d, align: (%s,),\n", columns, strings.Join(aligns, ", "))
	var head []string
	i := 0
	for ; i < len(rows) && rows[i].head; i++ {
		for _, td := range rows[i].cells {
			head = append(head, cell(td))
		}
	}
	if len(head) > 0 {
		s.WriteString("table.header(" + strings.Join(head, ", ") + "),\n")
	}
	for ; i < len(rows); i++ {
		for _, td := range rows[i].cells {
			s.WriteString(cell(td) + ", ")
		}
		s.WriteString("\n")
	}
	s.WriteString(")")
	return s.String()
}

func (c *typConv) figure(n *nethtml.Node) string {
	var images []string
	caption := ""
	var walk func(el *nethtml.Node)
	walk = func(el *nethtml.Node) {
		for k := el.FirstChild; k != nil; k = k.NextSibling {
			switch {
			case k.Type != nethtml.ElementNode:
			case k.Data == "figcaption":
				caption = strings.TrimSpace(c.inlines(k))
			case k.Data == "img":
				if s := c.image(k, true); s != "" {
					images = append(images, s)
				}
			case k.Namespace == "svg" && k.Data == "svg":
				images = append(images, c.svg(k, true))
			case k.Data == "pre":
				images = append(images, c.code(k))
			default:
				walk(k)
			}
		}
	}
	walk(n)
	if len(images) == 0 {
		return caption
	}
	return "#octavo-figure([" + strings.Join(images, "\n\n") + "], [" + caption + "])"
}

var imageWidth = regexp.MustCompile(`width:\s*([0-9.]+)(%|px|em|rem)`)

// image writes an <img>: as a block when it stands alone, else in the line.
func (c *typConv) image(img *nethtml.Node, block bool) string {
	src := xhtml.Attr(img, "src")
	t, ok := c.resolver.Resolve(c.ch.Path, src)
	if !ok {
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			c.warn("image %s is on the web, left out of the PDF", src)
		}
		return ""
	}
	file := filepath.Join(c.book.Dir, filepath.FromSlash(t.Path))
	if _, err := os.Stat(file); err != nil || !filepath.IsLocal(filepath.FromSlash(t.Path)) {
		c.warn("image %s not found, left out of the PDF", t.Path)
		return ""
	}
	switch strings.ToLower(filepath.Ext(file)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp":
	default:
		c.warn("image %s: Typst cannot show %s files, left out of the PDF", t.Path, filepath.Ext(file))
		return ""
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return ""
	}
	p := filepath.ToSlash(strings.TrimPrefix(abs, filepath.VolumeName(abs)))
	if !block {
		return "#box(height: 1.1em, baseline: 20%, image(" + typStr(p) + "));"
	}
	width := "auto"
	if m := imageWidth.FindStringSubmatch(xhtml.Attr(img, "style") + ";width:" + xhtml.Attr(img, "width") + "px"); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		switch m[2] {
		case "%":
			width = strconv.FormatFloat(min(v, 100), 'f', -1, 64) + "%"
		case "px":
			width = strconv.FormatFloat(v*0.75, 'f', -1, 64) + "pt"
		default:
			width = m[1] + "em"
		}
	}
	return "#octavo-img(" + typStr(p) + ", width: " + width + ")"
}

// svg writes an inline <svg> to a file beside book.typ and shows it.
func (c *typConv) svg(n *nethtml.Node, block bool) string {
	if xhtml.Attr(n, "xmlns") == "" {
		n.Attr = append(n.Attr, nethtml.Attribute{Key: "xmlns", Val: "http://www.w3.org/2000/svg"})
	}
	var buf bytes.Buffer
	if err := nethtml.Render(&buf, n); err != nil {
		return ""
	}
	c.svgs++
	name := fmt.Sprintf("svg/%d.svg", c.svgs)
	if err := os.MkdirAll(filepath.Join(c.dir, "svg"), 0o755); err != nil {
		return ""
	}
	if err := os.WriteFile(filepath.Join(c.dir, filepath.FromSlash(name)), buf.Bytes(), 0o644); err != nil {
		return ""
	}
	if !block {
		return "#box(height: 1.1em, image(" + typStr(name) + "));"
	}
	return "#octavo-img(" + typStr(name) + ", width: auto)"
}

func (c *typConv) inline(n *nethtml.Node) string {
	switch n.Type {
	case nethtml.TextNode:
		return typText(spaces.ReplaceAllString(n.Data, " "))
	case nethtml.ElementNode:
	default:
		return ""
	}
	if n.Namespace == "svg" && n.Data == "svg" {
		return c.svg(n, false)
	}
	mark := ""
	if l := c.label(n); l != "" {
		mark = "#metadata(none) " + l + " "
	}
	wrap := func(fn string) string { return mark + "#" + fn + "[" + c.inlines(n) + "];" }
	switch n.Data {
	case "script", "style", "template", "noscript", "iframe", "video", "audio", "object", "button", "select", "textarea":
		return ""
	case "strong", "b":
		return wrap("strong")
	case "em", "i", "cite", "dfn", "var":
		return wrap("emph")
	case "del", "s", "strike":
		return wrap("strike")
	case "u", "ins":
		return wrap("underline")
	case "mark":
		return wrap("highlight")
	case "sub":
		return wrap("sub")
	case "small":
		return mark + "#text(size: 0.85em)[" + c.inlines(n) + "];"
	case "kbd":
		return wrap("octavo-kbd")
	case "sup":
		if a := footnoteRef(n); a != nil {
			return c.footnote(a)
		}
		return wrap("super")
	case "code", "samp", "tt":
		return mark + "#raw(" + typStr(breakLong(spaces.ReplaceAllString(textOf(n), " "), 24)) + ");"
	case "br":
		return "#linebreak();"
	case "img":
		return mark + c.image(n, false)
	case "input":
		if xhtml.Attr(n, "type") == "checkbox" {
			for _, a := range n.Attr {
				if a.Key == "checked" {
					return "#octavo-box(true);"
				}
			}
			return "#octavo-box(false);"
		}
		return ""
	case "a":
		if xhtml.HasClass(n, "footnote-backref") {
			return ""
		}
		if xhtml.HasClass(n, "footnote-ref") {
			return c.footnote(n)
		}
		return mark + c.link(n)
	}
	return mark + c.inlines(n)
}

func footnoteRef(sup *nethtml.Node) *nethtml.Node {
	for k := sup.FirstChild; k != nil; k = k.NextSibling {
		if k.Type == nethtml.ElementNode && k.Data == "a" && xhtml.HasClass(k, "footnote-ref") {
			return k
		}
	}
	return nil
}

// footnote places a real footnote at the reference. A note referred to
// twice is placed once and pointed to the second time.
func (c *typConv) footnote(a *nethtml.Node) string {
	id := strings.TrimPrefix(xhtml.Attr(a, "href"), "#")
	if l, ok := c.noted[id]; ok {
		return "#footnote(<" + l + ">);"
	}
	li := c.footnotes[id]
	if li == nil {
		return "#super[" + typText(textOf(a)) + "];"
	}
	l := typLabel(c.prefix + id)
	c.noted[id] = l
	body := c.blocks(li)
	return "#footnote[" + strings.TrimSpace(body) + "]<" + l + ">"
}

func (c *typConv) link(a *nethtml.Node) string {
	text := c.inlines(a)
	href := xhtml.Attr(a, "href")
	switch {
	case href == "":
		return text
	case strings.HasPrefix(href, "#"):
		if l := typLabel(c.prefix + href[1:]); c.labels[l] {
			return "#link(<" + l + ">)[" + text + "];"
		}
		return text
	case strings.HasPrefix(href, "http://"), strings.HasPrefix(href, "https://"), strings.HasPrefix(href, "mailto:"):
		return "#link(" + typStr(href) + ")[" + text + "];"
	}
	if t, ok := c.resolver.Resolve(c.ch.Path, href); ok && t.Chapter != nil {
		l := c.anchor(t.Index)
		if t.Fragment != "" {
			if fl := typLabel(l + "-" + t.Fragment); c.labels[fl] {
				l = fl
			}
		}
		return "#link(<" + l + ">)[" + text + "];"
	}
	return text
}
