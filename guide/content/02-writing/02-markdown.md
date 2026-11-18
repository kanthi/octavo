# Markdown

Octavo reads [CommonMark](https://commonmark.org), the common standard for Markdown, with the additions GitHub uses. This page shows each one; take a [copy of this guide](../../index.md) to see how they are written.

## Text

*Italic*, **bold**, `code`, ~~struck out~~ and [links](https://example.com). A web address on its own, such as https://example.com, becomes a link too.

A blank line starts a new paragraph.

## Lists

- An item
- Another
  - Nested under it

1. First
2. Second

Task lists:

- [x] Write the chapter
- [ ] Read it again

## Tables

| Format | File | Needs |
|---|---|:---:|
| Website | `_book/index.html` | nothing |
| EPUB | `_book/my-book.epub` | nothing |
| PDF | `_book/my-book.pdf` | a browser |

The colons in the line under the header set the alignment of a column: `:---:` centres it, `---:` puts it on the right.

## Code

Name the language after the opening fence and the code is coloured:

```go
package main

import "fmt"

func main() {
	fmt.Println("Hello, book")
}
```

On the website every code block gets a **Copy** button. Code colours follow the light or dark theme; you can choose other colour styles (see [Book settings](../06-reference/02-book-settings.md)).

## Links between chapters

Link to another chapter by its Markdown file, relative to the chapter you are writing:

```markdown
See [the callouts](03-callouts.md), or [the PDF](../03-formats/04-pdf.md#pdf-needs-a-browser).
```

Octavo changes `.md` to the right address in each format, so the link works on the website, in the EPUB and in the PDF. A `#` part points at a heading in that chapter: the heading's text in lower case, with spaces as hyphens.

## Images

```markdown
![What the picture shows, for readers who cannot see it](../images/diagram.svg)
```

Images are found relative to the chapter, and every image a chapter uses is copied into the built book. SVG, PNG, JPEG, GIF and WebP all work. Always write the text in the brackets: screen readers read it out, and it shows when an image cannot load.

## Footnotes

Put a marker where the note belongs,[^marker] and the note itself anywhere in the chapter:

```markdown
...where the note belongs,[^marker] and...

[^marker]: The note's text.
```

[^marker]: Like this one. On the website footnotes are listed at the end of the chapter; most e-reader apps show them as a pop-up.

## Headings

```markdown
## A section of the chapter
### A part of that section
```

Use `##` and `###` inside a chapter. The single `#` is the chapter's title (see [Chapters and parts](01-chapters.md)).

## Raw HTML

HTML written straight into a chapter is passed through, for the rare thing Markdown cannot do:

```html
<details>
  <summary>Show the answer</summary>
  Forty-two.
</details>
```

<details>
  <summary>Show the answer</summary>
  Forty-two.
</details>

For the EPUB and PDF, Octavo tidies such HTML into well-formed XML, so a missing closing tag does not break the e-book.
