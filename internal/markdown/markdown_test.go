package markdown

import (
	"bytes"
	"strings"
	"testing"
)

func render(t *testing.T, src string) (*Document, string) {
	t.Helper()
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := d.Render(&b, RenderOptions{}); err != nil {
		t.Fatal(err)
	}
	return d, b.String()
}

func TestTitleAndFrontMatter(t *testing.T) {
	d, html := render(t, "---\ntitle: From front matter\nunnumbered: true\n---\n# From heading\n\nBody.\n\n## Part one\n")
	if d.Meta.Title != "From front matter" || !d.Meta.Unnumbered || d.Title != "From heading" {
		t.Errorf("got meta %+v, title %q", d.Meta, d.Title)
	}
	if strings.Contains(html, "<h1") {
		t.Errorf("leading h1 should be removed from the body:\n%s", html)
	}
	if len(d.Headings) != 1 || d.Headings[0].ID != "part-one" || d.Headings[0].Text != "Part one" {
		t.Errorf("headings: %+v", d.Headings)
	}
}

func TestCallouts(t *testing.T) {
	_, html := render(t, "> [!TIP]\n> First line\n> second line.\n>\n> Another paragraph.\n\n> [!note]\n> Not a callout: the marker is lower case.\n\n> Plain quote.\n")
	for _, want := range []string{`<div class="callout callout-tip" role="note">`, `<p class="callout-title">Tip</p>`, "First line\nsecond line.", "<p>Another paragraph.</p>"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in:\n%s", want, html)
		}
	}
	if strings.Contains(html, "[!TIP]") {
		t.Errorf("marker left in output:\n%s", html)
	}
	if strings.Count(html, "<blockquote>") != 2 {
		t.Errorf("want 2 ordinary block quotes:\n%s", html)
	}
}

func TestLinksImagesAndText(t *testing.T) {
	d, _ := render(t, "See [next](two.md#part) and ![pic](img/a.png).\n\n```go\nfmt.Println(1)\n```\n")
	d.RewriteLinks(func(dest string) string { return strings.Replace(dest, ".md", ".html", 1) })
	var b bytes.Buffer
	d.Render(&b, RenderOptions{})
	if !strings.Contains(b.String(), `href="two.html#part"`) {
		t.Errorf("link not rewritten:\n%s", b.String())
	}
	if len(d.Images) != 1 || d.Images[0] != "img/a.png" {
		t.Errorf("images: %v", d.Images)
	}
	if text := d.PlainText(); !strings.Contains(text, "See next and") || !strings.Contains(text, "fmt.Println(1)") {
		t.Errorf("plain text: %q", text)
	}
}
