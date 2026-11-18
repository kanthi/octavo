// Package pdf renders a book as one PDF file.
//
// Octavo does not typeset PDF itself. It hands the book to one of two
// engines (see "engine"):
//
//   - Typst, a typesetting program: the book is written as Typst markup
//     (typst.go). It gives page numbers in the contents, footnotes at the
//     foot of the page, and clean chapter-opening pages.
//   - A web browser: the book is written as a single HTML page with print
//     styles and printed by Chrome, Chromium, Edge or Brave, run without a
//     window. Any other HTML-to-PDF tool can be used through "command".
//
// The page is laid out as a book, not as a printed web page: a title page
// and contents with roman page numbers, a page for each part, every chapter
// on a new page, the chapter title as a running header, justified and
// hyphenated text in a book typeface, and tables and code that break cleanly
// across pages.
package pdf

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	nethtml "golang.org/x/net/html"
	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/format"
	"github.com/kanthi/octavo/internal/format/fonts"
	"github.com/kanthi/octavo/internal/format/xhtml"
	"github.com/kanthi/octavo/internal/markdown"
)

//go:embed pdf.css
var pdfCSS string

// Settings are the keys under "format: pdf:" in octavo.yml.
type Settings struct {
	OutputFile     string            `yaml:"output-file"`     // default: the book title, such as my-book.pdf
	Paper          string            `yaml:"paper"`           // A4 (default), letter, A5, or a size such as "6in 9in"
	Margin         string            `yaml:"margin"`          // page margins in CSS form, default "26mm 25mm 28mm"
	Font           string            `yaml:"font"`            // literata (default), source-serif, atkinson, or system
	FontSize       string            `yaml:"font-size"`       // default 10.5pt
	Justify        *bool             `yaml:"justify"`         // justified text with hyphenation, default true
	Headers        *bool             `yaml:"headers"`         // the chapter title at the top of each page, default true
	TOC            *bool             `yaml:"toc"`             // a contents page, default true
	PageNumbers    *bool             `yaml:"page-numbers"`    // default true
	CSS            config.StringList `yaml:"css"`             // your stylesheets, added after the built-in one
	Highlight      string            `yaml:"highlight"`       // chroma style name for code
	NumberChapters *bool             `yaml:"number-chapters"` // default true

	// Browser is the Chrome, Chromium, Edge or Brave program to print with.
	// Default: $OCTAVO_BROWSER, then the first one found on this machine.
	Browser string `yaml:"browser"`
	// Command replaces the browser with any HTML-to-PDF tool. "{input}" is
	// the HTML file and "{output}" the PDF to write, for example:
	// weasyprint {input} {output}
	Command string `yaml:"command"`
	// Timeout is how long the tool may take, in seconds. Default 300.
	Timeout int `yaml:"timeout"`

	// Engine picks what makes the PDF: "typst", "chrome" (any Chrome-family
	// browser), or "auto" (the default): Typst when it is installed, else
	// the browser. $OCTAVO_PDF_ENGINE overrides it. "command" overrides both.
	Engine string `yaml:"engine"`
	// Typst is the Typst program. Default: $OCTAVO_TYPST, then "typst".
	Typst string `yaml:"typst"`
}

// Engines, as named in "engine:".
const (
	engineAuto    = "auto"
	engineTypst   = "typst"
	engineChrome  = "chrome"
	engineCommand = "command"
)

type Format struct{ s Settings }

// New is the format.Factory for PDF.
func New(node yaml.Node) (format.Format, error) {
	s := Settings{Paper: "A4", Margin: "26mm 25mm 28mm", Font: "literata", FontSize: "10.5pt", Highlight: "github", Timeout: 300}
	if err := config.Decode(node, &s); err != nil {
		return nil, err
	}
	if s.OutputFile != "" && !strings.HasSuffix(s.OutputFile, ".pdf") {
		s.OutputFile += ".pdf"
	}
	if _, ok := fonts.Find(s.Font); !ok && s.Font != "system" {
		return nil, fmt.Errorf("font must be %s or system, not %q", strings.Join(fonts.Keys(), ", "), s.Font)
	}
	switch s.Engine {
	case "", engineAuto, engineTypst, engineChrome:
	default:
		return nil, fmt.Errorf("engine must be auto, typst or chrome, not %q", s.Engine)
	}
	for name, value := range map[string]string{"paper": s.Paper, "margin": s.Margin, "font-size": s.FontSize} {
		if strings.ContainsAny(value, ";{}") {
			return nil, fmt.Errorf("%s: %q is not a CSS size", name, value)
		}
	}
	return &Format{s: s}, nil
}

var esc = html.EscapeString

// File is the name of the PDF in the output folder.
func (f *Format) File(b *book.Book) string {
	if f.s.OutputFile != "" {
		return f.s.OutputFile
	}
	return xhtml.FileName(b.Config.Book.Title, ".pdf")
}

func on(setting *bool) bool { return setting == nil || *setting }

// Check reports whether a PDF can be made on this machine.
func (f *Format) Check() error {
	_, _, err := f.engine()
	return err
}

// engine returns the engine that will make the PDF and its program.
func (f *Format) engine() (kind, tool string, err error) {
	if f.s.Command != "" {
		return engineCommand, f.s.Command, nil
	}
	want := strings.ToLower(strings.TrimSpace(os.Getenv("OCTAVO_PDF_ENGINE")))
	if want == "" {
		want = f.s.Engine
	}
	switch want {
	case engineTypst:
		p, err := FindTypst(f.s.Typst)
		if err != nil {
			return "", "", fmt.Errorf("engine is typst, but %w; install it or set \"engine: auto\"", err)
		}
		return engineTypst, p, nil
	case engineChrome:
		p, err := FindBrowser(f.s.Browser)
		return engineChrome, p, err
	case "", engineAuto:
	default:
		return "", "", fmt.Errorf("OCTAVO_PDF_ENGINE must be auto, typst or chrome, not %q", want)
	}
	// Auto. A book with its own PDF stylesheets was styled for the browser,
	// so it keeps the browser when there is one.
	if len(f.s.CSS) == 0 {
		if p, err := FindTypst(f.s.Typst); err == nil {
			return engineTypst, p, nil
		}
	}
	if p, err := FindBrowser(f.s.Browser); err == nil {
		return engineChrome, p, nil
	}
	if p, err := FindTypst(f.s.Typst); err == nil {
		return engineTypst, p, nil
	}
	return "", "", fmt.Errorf("PDF needs Typst or a Chrome-family browser (Chrome, Chromium, Edge or Brave), and neither was found; " +
		"install Typst (https://typst.app, \"brew install typst\") or a browser, point to one with \"format: pdf: typst:\" or \"browser:\", " +
		"or set \"format: pdf: command:\" to another HTML-to-PDF tool")
}

// FindTypst returns the Typst program: the given one, else $OCTAVO_TYPST,
// else "typst" on the PATH.
func FindTypst(setting string) (string, error) {
	for _, name := range []string{setting, os.Getenv("OCTAVO_TYPST")} {
		if name == "" {
			continue
		}
		p, err := exec.LookPath(name)
		if err != nil {
			return "", fmt.Errorf("Typst program %q not found", name)
		}
		return p, nil
	}
	p, err := exec.LookPath("typst")
	if err != nil {
		return "", fmt.Errorf("Typst was not found")
	}
	return p, nil
}

// cssString writes text as a CSS string, for the running headers.
func cssString(text string) string {
	text = strings.NewReplacer(`\\`, `\\\\`, `"`, `\\"`, "\n", " ", "\r", " ").Replace(text)
	return `"` + text + `"`
}

func (f *Format) Render(b *book.Book, out *format.Output, opts format.Options) error {
	kind, tool, err := f.engine()
	if err != nil {
		return err
	}
	if kind == engineTypst {
		return f.renderTypst(b, out, opts, tool)
	}
	name := f.File(b)
	doc, err := f.document(b, opts)
	if err != nil {
		return err
	}
	// The PDF depends on the page below and on the image files it shows.
	fingerprint := imagesFingerprint(b, format.Hash(doc, kind, f.s.Browser, f.s.Command))
	if out.Fresh(name, fingerprint) {
		return nil
	}

	tmp, err := os.MkdirTemp("", "octavo-pdf-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	input, output := filepath.Join(tmp, "book.html"), filepath.Join(tmp, "book.pdf")
	if err := os.WriteFile(input, doc, 0o644); err != nil {
		return err
	}
	// The page refers to its fonts as fonts/<file>, beside it.
	which := []fonts.Font{fonts.Mono}
	if font, ok := fonts.Find(f.s.Font); ok {
		which = append(which, font)
	}
	if err := fonts.WriteTo(filepath.Join(tmp, "fonts"), which...); err != nil {
		return err
	}
	if err := f.print(tmp, input, output); err != nil {
		return err
	}
	data, err := os.ReadFile(output)
	if err != nil || len(data) == 0 {
		return fmt.Errorf("the PDF tool finished but wrote no file")
	}
	return out.Write(name, fingerprint, data)
}

// imagesFingerprint adds the image files a book shows to a fingerprint, so
// that changing an image makes the PDF again.
func imagesFingerprint(b *book.Book, fingerprint uint64) uint64 {
	for _, ch := range b.Chapters {
		for _, img := range ch.Doc.Images {
			if info, err := os.Stat(filepath.Join(b.Dir, filepath.Dir(filepath.FromSlash(ch.Path)), filepath.FromSlash(img))); err == nil {
				fingerprint = format.Hash(fingerprint, img, info.ModTime().UnixNano(), info.Size())
			}
		}
	}
	return fingerprint
}

// document writes the whole book as one HTML page made for printing.
func (f *Format) document(b *book.Book, opts format.Options) ([]byte, error) {
	info := b.Config.Book
	numbered := on(f.s.NumberChapters)
	resolver := xhtml.NewResolver(b)
	anchor := func(i int) string { return fmt.Sprintf("ch%d", i+1) }
	index := map[*book.Chapter]int{}
	for i, ch := range b.Chapters {
		index[ch] = i
	}

	var s strings.Builder
	s.WriteString("<!doctype html>\n<html lang=\"" + esc(info.Lang) + "\">\n<head>\n<meta charset=\"utf-8\">\n<title>" + esc(info.Title) + "</title>\n")
	if info.Author != "" {
		s.WriteString(`<meta name="author" content="` + esc(info.Author) + "\">\n")
	}
	// Pages. Front matter is numbered i, ii, iii; a part page carries no
	// header; each chapter has its own page style so that its title can run
	// along the top of its pages.
	number := func(style string) string {
		if !on(f.s.PageNumbers) {
			return ""
		}
		return ` @bottom-center { content: counter(page` + style + `); font: 9pt var(--pdf-font); color: #555; }`
	}
	family := `Charter, "Bitstream Charter", "Iowan Old Style", Georgia, "Times New Roman", serif`
	s.WriteString("<style>\n")
	s.WriteString(fonts.CSS("fonts/", fonts.Mono))
	if font, ok := fonts.Find(f.s.Font); ok {
		s.WriteString(fonts.CSS("fonts/", font))
		family = fmt.Sprintf("%q, %s", font.Family, family)
		if font.Key == "atkinson" {
			family = fmt.Sprintf("%q, system-ui, sans-serif", font.Family)
		}
	}
	align := "justify"
	if !on(f.s.Justify) {
		align = "left"
	}
	s.WriteString(":root { --pdf-font: " + family + "; --pdf-size: " + f.s.FontSize + "; --pdf-align: " + align + "; }\n")
	s.WriteString("@page { size: " + f.s.Paper + "; margin: " + f.s.Margin + ";" + number("") + " }\n")
	s.WriteString("@page front {" + number(", lower-roman") + " }\n")
	s.WriteString("@page part { }\n@page :first { @bottom-center { content: none; } }\n")
	if on(f.s.Headers) {
		header := ` font: italic 8.5pt var(--pdf-font); color: #555; vertical-align: bottom; padding-bottom: 4mm; }`
		for i, ch := range b.Chapters {
			left := info.Title
			if ch.Title == info.Title { // the preface is often named after the book; say it once
				left = ""
			}
			fmt.Fprintf(&s, "@page %s { @top-left { content: %s;%s @top-right { content: %s;%s }\n.%s { page: %s; }\n",
				anchor(i), cssString(left), header, cssString(xhtml.Label(ch, numbered)), header, anchor(i), anchor(i))
		}
	}
	s.WriteString(xhtml.ContentCSS + pdfCSS + xhtml.HighlightCSS(f.s.Highlight))
	for _, file := range f.s.CSS {
		data, err := os.ReadFile(filepath.Join(b.Dir, file))
		if err != nil {
			return nil, fmt.Errorf("css: %w", err)
		}
		// A stylesheet cannot contain "</style", so this cannot end the block early by accident.
		s.WriteString("\n" + strings.ReplaceAll(string(data), "</style", `<\/style`))
	}
	s.WriteString("\n</style>\n</head>\n<body>\n")

	s.WriteString(`<section class="front title-page">` + "\n" + `<p class="book-title">` + esc(info.Title) + "</p>\n" + `<div class="title-rule"></div>` + "\n")
	if info.Author != "" {
		s.WriteString(`<p class="author">` + esc(info.Author) + "</p>\n")
	}
	if info.Date != "" {
		s.WriteString(`<p class="date">` + esc(info.Date) + "</p>\n")
	}
	if info.Description != "" {
		s.WriteString(`<p class="description">` + esc(info.Description) + "</p>\n")
	}
	s.WriteString("</section>\n")

	if on(f.s.TOC) {
		var toc func(nodes []*book.Node) string
		toc = func(nodes []*book.Node) string {
			var t strings.Builder
			for _, n := range nodes {
				switch {
				case n.Chapter != nil:
					num := ""
					if numbered && n.Chapter.Number > 0 {
						num = fmt.Sprintf(`<span class="number">%d</span>`, n.Chapter.Number)
					}
					t.WriteString(`<li><a href="#` + anchor(index[n.Chapter]) + `">` + num + esc(n.Title) + "</a></li>\n")
				case len(n.Children) > 0:
					t.WriteString(`<li><span class="group">` + esc(n.Title) + "</span>\n<ol>\n" + toc(n.Children) + "</ol></li>\n")
				}
			}
			return t.String()
		}
		s.WriteString(`<nav class="front toc">` + "\n<h1>Contents</h1>\n<ol>\n" + toc(b.Nav) + "</ol>\n</nav>\n")
	}

	// Chapters in reading order. A top-level part gets a page of its own.
	parts := 0
	var walk func(nodes []*book.Node, depth int) error
	walk = func(nodes []*book.Node, depth int) error {
		for _, n := range nodes {
			if n.Chapter == nil {
				if depth == 0 && len(n.Children) > 0 {
					parts++
					fmt.Fprintf(&s, `<section class="part"><p class="part-label">Part %d</p><h1>%s</h1></section>`+"\n", parts, esc(n.Title))
				}
				if err := walk(n.Children, depth+1); err != nil {
					return err
				}
				continue
			}
			ch, i := n.Chapter, index[n.Chapter]
			// All chapters share one page, so ids get the chapter's name in
			// front ("ch3-setup") to keep two "#setup" headings apart.
			prefix := anchor(i) + "-"
			body, err := xhtml.Body(ch.Doc, markdown.RenderOptions{Format: "pdf", FigurePrefix: xhtml.FigurePrefix(ch, numbered)}, func(el *nethtml.Node) {
				if id := xhtml.Attr(el, "id"); id != "" {
					xhtml.SetAttr(el, "id", prefix+id)
				}
				switch el.Data {
				case "a":
					href := xhtml.Attr(el, "href")
					if strings.HasPrefix(href, "#") && len(href) > 1 {
						xhtml.SetAttr(el, "href", "#"+prefix+href[1:])
					} else if t, ok := resolver.Resolve(ch.Path, href); ok && t.Chapter != nil {
						target := "#" + anchor(t.Index)
						if t.Fragment != "" {
							target += "-" + t.Fragment
						}
						xhtml.SetAttr(el, "href", target)
					}
				case "img":
					if t, ok := resolver.Resolve(ch.Path, xhtml.Attr(el, "src")); ok {
						file := filepath.Join(b.Dir, filepath.FromSlash(t.Path))
						if _, err := os.Stat(file); err != nil || !filepath.IsLocal(filepath.FromSlash(t.Path)) {
							fmt.Fprintf(opts.Log, "warning: %s: image %s not found, left out of the PDF\n", ch.Path, t.Path)
							return
						}
						xhtml.SetAttr(el, "src", (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String())
					}
				}
			})
			if err != nil {
				return fmt.Errorf("chapter %s: %w", ch.Path, err)
			}
			s.WriteString(`<section class="chapter ` + anchor(i) + `" id="` + anchor(i) + `">` + "\n<h1>")
			if numbered && ch.Number > 0 {
				fmt.Fprintf(&s, `<span class="chapter-number">Chapter %d</span>`, ch.Number)
			}
			s.WriteString(esc(ch.Title) + "</h1>\n")
			s.Write(body)
			s.WriteString("\n</section>\n")
		}
		return nil
	}
	if err := walk(b.Nav, 0); err != nil {
		return nil, err
	}
	s.WriteString("</body>\n</html>\n")
	return []byte(s.String()), nil
}

// print turns the HTML file input into the PDF file output.
func (f *Format) print(tmp, input, output string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(f.s.Timeout)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	var tool string
	if f.s.Command != "" {
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
		tool = f.s.Command
		cmd = exec.CommandContext(ctx, "sh", "-c", strings.NewReplacer("{input}", quote(input), "{output}", quote(output)).Replace(f.s.Command))
	} else {
		browser, err := FindBrowser(f.s.Browser)
		if err != nil {
			return err
		}
		tool = browser
		args := []string{
			"--headless", "--disable-gpu", "--no-first-run", "--no-default-browser-check", "--disable-extensions",
			"--user-data-dir=" + filepath.Join(tmp, "profile"),   // never touch the user's own browser profile
			"--no-pdf-header-footer", "--print-to-pdf-no-header", // no date and file name in the page margins
			"--generate-pdf-document-outline", // headings become PDF bookmarks
			"--print-to-pdf=" + output,
		}
		if os.Geteuid() == 0 {
			args = append(args, "--no-sandbox") // browsers refuse to start as root otherwise (containers, CI)
		}
		cmd = exec.CommandContext(ctx, browser, append(args, (&url.URL{Scheme: "file", Path: filepath.ToSlash(input)}).String())...)
	}
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s took longer than %d seconds (raise \"timeout\" under format: pdf:)", tool, f.s.Timeout)
		}
		return fmt.Errorf("%s: %w\n%s", tool, err, lastLines(log.String(), 8))
	}
	return nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// FindBrowser returns the browser to print with: the given one, else
// $OCTAVO_BROWSER, else the first Chrome-family browser on this machine.
func FindBrowser(setting string) (string, error) {
	for _, name := range []string{setting, os.Getenv("OCTAVO_BROWSER")} {
		if name == "" {
			continue
		}
		p, err := exec.LookPath(name)
		if err != nil {
			return "", fmt.Errorf("browser %q not found", name)
		}
		return p, nil
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome", "microsoft-edge", "brave-browser"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	var places []string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		for _, app := range []string{"Google Chrome", "Chromium", "Microsoft Edge", "Brave Browser"} {
			for _, root := range []string{"/Applications", filepath.Join(home, "Applications")} {
				places = append(places, filepath.Join(root, app+".app", "Contents", "MacOS", app))
			}
		}
	case "windows":
		for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")} {
			if root != "" {
				places = append(places,
					filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"),
					filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"))
			}
		}
	}
	for _, p := range places {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("PDF needs Chrome, Chromium, Edge or Brave to print the book, and none was found; " +
		"install one, point to it with OCTAVO_BROWSER or \"format: pdf: browser:\", or set \"format: pdf: command:\" to another HTML-to-PDF tool")
}
