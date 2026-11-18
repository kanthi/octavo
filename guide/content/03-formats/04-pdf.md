# The PDF

The PDF is laid out as a book, not as a printed web page:

- A title page and a contents page, numbered i, ii, iii; a page for each part; every chapter starting on a new page.
- The book's title and the chapter's title along the top of each page, and page numbers at the foot.
- Justified, hyphenated text in Literata at 10.5 points. Headings, lists, tables and code are never justified.
- Tables with rules in the style of printed books; their header rows repeat when a table runs over a page.
- Code in JetBrains Mono that wraps instead of running off the page.
- Bookmarks for every chapter, and links between chapters that work.

## Two engines: Typst and a browser

Octavo does not set the pages itself. It hands the book to one of two programs:

| | Typst | A browser |
|---|---|---|
| What it is | A typesetting program, [typst.app](https://typst.app) | Chrome, Chromium, Edge or Brave, run without a window |
| Page numbers on the contents page | Yes | No |
| Footnotes | At the foot of the page | At the end of the chapter |
| Running titles | Not on a chapter's first page | On every page |
| Long lines of code | Set smaller to fit | Wrapped |
| Your PDF stylesheets (`css:`) | Not used | Used |
| Speed | Fast: a 450-chapter book in seconds | About ten times slower |

The `engine` setting picks one:

```yaml
format:
  pdf:
    engine: auto    # the default: Typst when it is installed, else a browser
```

- `auto` uses Typst when it is installed, else a browser. A book with its own PDF stylesheets (`css:`) uses the browser first, because only the browser reads them.
- `typst` always uses Typst, and stops with an error when it is missing.
- `chrome` always uses a browser.

`OCTAVO_PDF_ENGINE=typst` (or `chrome`, `auto`) does the same for one run, whatever `octavo.yml` says.

### Finding Typst

Octavo uses `typst:` under `format: pdf:` in `octavo.yml`, else the `OCTAVO_TYPST` environment variable, else the `typst` program on your `PATH`.

### Finding a browser

It looks for one in this order:

1. `browser:` under `format: pdf:` in `octavo.yml`
2. The `OCTAVO_BROWSER` environment variable
3. The programs on your `PATH`
4. The usual install folders on macOS and Windows

With neither Typst nor a browser, a plain `octavo render` skips the PDF with a warning and builds the rest. `--to pdf` stops with an error instead.

To use another HTML-to-PDF tool, give its command line. `{input}` and `{output}` are replaced with the files:

```yaml
format:
  pdf:
    command: weasyprint {input} {output}
```

## Settings

```yaml
format:
  pdf:
    paper: A5
    font: source-serif
    font-size: 11pt
```

| Setting | Meaning | Default |
|---|---|---|
| `paper` | `A4`, `letter`, `A5`, or a size such as `6in 9in` | `A4` |
| `margin` | Page margins, in CSS form | `26mm 25mm 28mm` |
| `font` | `literata`, `source-serif`, `atkinson`, or `system` | `literata` |
| `font-size` | Text size | `10.5pt` |
| `justify` | Justified, hyphenated text | `true` |
| `headers` | Titles along the top of each page | `true` |
| `page-numbers` | Page numbers at the foot | `true` |
| `toc` | A contents page | `true` |
| `number-chapters` | Chapter numbers | `true` |
| `css` | Your stylesheets, after the built-in one (browser only) | |
| `highlight` | Code colour style (browser only) | `github` |
| `output-file` | The file name | The title, such as `my-book.pdf` |
| `engine` | `auto`, `typst` or `chrome` | `auto` |
| `typst` | The Typst program to use | Found by itself |
| `browser` | The browser program to use | Found by itself |
| `command` | Another HTML-to-PDF tool | |
| `timeout` | Seconds to wait for the browser or tool | `300` |

## Not yet in the PDF

- **Maths.** Formulas written in TeX (`$...$`) appear as TeX.
- **Code colours with Typst.** Typst colours code its own way; `highlight:` applies to the browser engine.
