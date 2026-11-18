package book

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/markdown"
)

// A chapter's source is expanded before it is parsed:
//
//	{{< include other.md >}}        on a line of its own: that file's text
//	{{< var name >}}                a value from "variables:" in octavo.yml
//	{{< meta title >}}              a book setting: title, author, date, ...
//	{{{< var name >}}}              written as is, without the outer braces
//	::: {.content-visible when-format="pdf"}
//	...                             only in the PDF ("unless-format",
//	:::                             ".content-hidden" and lists work too)
//
// Nothing inside fenced code blocks is expanded, so a chapter can show these
// forms in an example.

var (
	shortcode   = regexp.MustCompile(`\{(\{\{<|\{<)\s*(var|meta|include)\s+([^\s>]+)\s*>\}\}(\}?)`)
	includeLine = regexp.MustCompile(`^\s*\{\{<\s*include\s+([^\s>]+)\s*>\}\}\s*$`)
	formatOpen  = regexp.MustCompile(`^:::+\s*\{\s*\.content-(visible|hidden)\s+(when|unless)-format=["']([a-z, ]+)["']\s*\}\s*$`)
	divOpen     = regexp.MustCompile(`^:::+\s*\S`)
	divClose    = regexp.MustCompile(`^:::+\s*$`)
	fence       = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
)

// Formats a format block may name.
var formatNames = map[string]bool{"html": true, "epub": true, "pdf": true}

type expander struct {
	bookDir string
	vars    map[string]any
	meta    map[string]string
	files   []string // every file read, chapter first, relative to the book
}

func newExpander(bookDir string, cfg *config.Config) *expander {
	b := cfg.Book
	return &expander{bookDir: bookDir, vars: cfg.Variables, meta: map[string]string{
		"title": b.Title, "author": b.Author, "date": b.Date, "description": b.Description,
		"lang": b.Lang, "site-url": b.SiteURL, "created": b.Created,
	}}
}

// settingsKey changes whenever a setting that expansion uses changes, so the
// cache knows to expand again.
func (e *expander) settingsKey() string {
	keys := make([]string, 0, len(e.meta))
	for k, v := range e.meta {
		keys = append(keys, k+"="+v)
	}
	sort.Strings(keys)
	return fmt.Sprint(keys, e.vars)
}

// expand returns the source of the chapter at rel (relative to the book).
func (e *expander) expand(rel string) ([]byte, error) {
	e.files = nil
	return e.file(rel, nil)
}

func (e *expander) file(rel string, stack []string) ([]byte, error) {
	for _, s := range stack {
		if s == rel {
			return nil, fmt.Errorf("%s includes itself (%s)", rel, strings.Join(append(stack, rel), " → "))
		}
	}
	if len(stack) > 20 {
		return nil, fmt.Errorf("includes nested more than 20 deep at %s", rel)
	}
	src, err := os.ReadFile(filepath.Join(e.bookDir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	e.files = append(e.files, rel)
	stack = append(stack, rel)
	if len(stack) > 1 {
		src = dropFrontMatter(src)
	}

	var out bytes.Buffer
	var inFence string // the fence that opened the code block we are in
	var divs []bool    // open ::: blocks; true for format blocks
	lines := strings.SplitAfter(string(src), "\n")
	for i, line := range lines {
		where := fmt.Sprintf("%s:%d", rel, i+1)
		bare := strings.TrimRight(line, "\r\n")

		if m := fence.FindStringSubmatch(bare); m != nil {
			switch {
			case inFence == "":
				inFence = m[1]
			case m[1][0] == inFence[0] && len(m[1]) >= len(inFence) && strings.TrimSpace(bare[len(m[0]):]) == "":
				inFence = ""
			}
			out.WriteString(line)
			continue
		}
		if inFence != "" {
			out.WriteString(line)
			continue
		}

		if m := includeLine.FindStringSubmatch(bare); m != nil {
			target := filepath.ToSlash(filepath.Join(filepath.Dir(filepath.FromSlash(rel)), filepath.FromSlash(m[1])))
			if !filepath.IsLocal(filepath.FromSlash(target)) {
				return nil, fmt.Errorf("%s: include %s is outside the book folder", where, m[1])
			}
			inc, err := e.file(target, stack)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("%s: include %s: no such file", where, m[1])
				}
				return nil, err
			}
			out.WriteString("\n")
			out.Write(inc)
			out.WriteString("\n\n")
			continue
		}
		if m := formatOpen.FindStringSubmatch(bare); m != nil {
			var formats []string
			for _, f := range strings.Split(m[3], ",") {
				f = strings.TrimSpace(f)
				if !formatNames[f] {
					return nil, fmt.Errorf("%s: unknown format %q (html, epub or pdf)", where, f)
				}
				formats = append(formats, f)
			}
			show := (m[1] == "visible") == (m[2] == "when")
			divs = append(divs, true)
			out.WriteString("\n" + markdown.WhenComment(show, formats) + "\n\n")
			continue
		}
		if divClose.MatchString(bare) && len(divs) > 0 {
			ours := divs[len(divs)-1]
			divs = divs[:len(divs)-1]
			if ours {
				out.WriteString("\n" + markdown.EndComment + "\n\n")
				continue
			}
		} else if divOpen.MatchString(bare) {
			divs = append(divs, false) // some other ::: block: left as written
		}

		expanded, err := e.shortcodes(line, where)
		if err != nil {
			return nil, err
		}
		out.WriteString(expanded)
	}
	for _, ours := range divs {
		if ours {
			return nil, fmt.Errorf("%s: a \":::\" format block is not closed", rel)
		}
	}
	return out.Bytes(), nil
}

// shortcodes fills in {{< var >}} and {{< meta >}} on one line.
func (e *expander) shortcodes(line, where string) (string, error) {
	var err error
	out := shortcode.ReplaceAllStringFunc(line, func(s string) string {
		m := shortcode.FindStringSubmatch(s)
		if m[1] == "{{<" { // {{{< ... >}}}: written as is, minus one brace each side
			return s[1 : len(s)-1]
		}
		if m[4] != "" { // "{{< x >}}}": one brace too many; leave it for the author to see
			return s
		}
		kind, name := m[2], m[3]
		switch kind {
		case "var":
			v, ok := lookup(e.vars, name)
			if !ok {
				err = fmt.Errorf("%s: unknown variable %q (set it under \"variables:\" in octavo.yml)", where, name)
				return s
			}
			return v
		case "meta":
			v, ok := e.meta[name]
			if !ok {
				err = fmt.Errorf("%s: unknown book setting %q for meta (title, author, date, description, lang, site-url, created)", where, name)
				return s
			}
			return v
		}
		err = fmt.Errorf("%s: an include must be on a line of its own", where)
		return s
	})
	return out, err
}

// lookup finds "a.b.c" in nested maps.
func lookup(vars map[string]any, name string) (string, bool) {
	var cur any = vars
	for _, part := range strings.Split(name, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		if cur, ok = m[part]; !ok {
			return "", false
		}
	}
	switch cur.(type) {
	case map[string]any, []any, nil:
		return "", false
	}
	return fmt.Sprint(cur), true
}

func dropFrontMatter(src []byte) []byte {
	if !bytes.HasPrefix(src, []byte("---\n")) && !bytes.HasPrefix(src, []byte("---\r\n")) {
		return src
	}
	rest := src[bytes.IndexByte(src, '\n')+1:]
	for off := 0; off < len(rest); {
		line, next := rest[off:], len(rest)
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line, next = line[:i], off+i+1
		}
		if string(bytes.TrimRight(line, " \t\r")) == "---" {
			return rest[next:]
		}
		off = next
	}
	return src
}
