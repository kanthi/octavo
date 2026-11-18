# Book settings

A book's settings are in `octavo.yml`, at the top of the book's folder. Only `title` and `chapters` are required.

```yaml
book:
  title: My Book
  author: Your Name
  date: "2026-11-17"
  description: One sentence about the book.
  lang: en
  chapters:
    - index.md
    - chapters/01-intro.md

output-dir: _book

format:
  html: {}
  epub: {}
  pdf: {}

publish:
  gh-pages: {}
```

## book

| Key | Meaning | Default |
|---|---|---|
| `title` | The book's title. **Required.** | |
| `author` | The author, shown on the title page and in the files' details | |
| `date` | Any date text, shown under the title | |
| `description` | A sentence about the book: for search engines, the portal card, and the files' details | |
| `lang` | The language, as a code such as `en`, `de` or `pt-BR` | `en` |
| `draft` | `true` keeps the book out of builds unless `--drafts` is given | `false` |
| `chapters` | The chapter list. **Required.** See [Chapters and parts](../02-writing/01-chapters.md). | |
| `auto-add` | `false` to build only the chapters listed, without adding new numbered files | `true` |

## output-dir

Where `octavo render` puts the built book, relative to the book's folder. Default: `_book`. It must not be the book's own folder, because each full build empties it.

## format

One section per format. Leave a format out, or set it to `false`, to not build it. With no `format:` section at all, every format is built.

### html

| Key | Meaning | Default |
|---|---|---|
| `theme` | `light`, `dark`, or `auto` (follows the reader's system) | `auto` |
| `font` | `literata`, `source-serif`, `atkinson`, or `system` | `literata` |
| `css` | Your stylesheets, loaded after the built-in theme | |
| `page-toc` | `false` removes the "On this page" list | `true` |
| `number-chapters` | `false` for no chapter numbers | `true` |
| `sidebar-numbers` | `true` to show chapter numbers in the chapter list too | `false` |
| `highlight-light`, `highlight-dark` | Code colour styles | `github`, `github-dark` |
| `resources` | Other files to copy, as patterns | |
| `template` | Your own page template | |
| `offline` | `false` turns off reading offline | `true` |
| `redirects` | Old page addresses and where they now lead: a chapter file, maybe with `#section`, or a full address | |

See [The website](../03-formats/02-website.md).

### epub

| Key | Meaning | Default |
|---|---|---|
| `cover` | A PNG or JPEG image, or `none` | A drawn cover |
| `output-file` | The file name | From the title |
| `css` | Your stylesheets | |
| `highlight`, `highlight-dark` | Code colour styles | `github`, `github-dark` |
| `number-chapters` | `false` for no chapter numbers | `true` |

See [The EPUB](../03-formats/03-epub.md).

### pdf

| Key | Meaning | Default |
|---|---|---|
| `paper` | `A4`, `letter`, `A5`, or a size such as `6in 9in` | `A4` |
| `engine` | What sets the pages: `typst`, `chrome`, or `auto` (Typst when installed, else a browser) | `auto` |
| `margin` | Page margins, in CSS form | `26mm 25mm 28mm` |
| `font` | `literata`, `source-serif`, `atkinson`, or `system` | `literata` |
| `font-size` | Text size | `10.5pt` |
| `justify` | Justified, hyphenated text | `true` |
| `headers` | Titles at the top of each page | `true` |
| `page-numbers` | Page numbers | `true` |
| `toc` | A contents page | `true` |
| `number-chapters` | Chapter numbers | `true` |
| `css` | Your stylesheets | |
| `highlight` | Code colour style | `github` |
| `output-file` | The file name | From the title |
| `typst` | The Typst program | Found by itself |
| `browser` | The browser program | Found by itself |
| `command` | Another HTML-to-PDF tool, with `{input}` and `{output}` | |
| `timeout` | Seconds to wait for the browser or tool | `300` |

See [The PDF](../03-formats/04-pdf.md).

## versions

Versions of the book built side by side, such as editions, each in its own folder with a menu to switch. The first is the latest. See [Versions](../05-publishing/02-versions.md).

| Key | Meaning | Default |
|---|---|---|
| `label` | The name in the menu. **Required.** | |
| `ref` | A git branch, tag or commit to build this version from. Leave it out for the book as it is now. | |
| `folder` | Its folder in the output | Made from the label |

## publish

Named publish targets. See [Publishing](../05-publishing/01-publishing.md) and the page for each kind: [gh-pages](../05-publishing/targets/gh-pages.md), [ftp](../05-publishing/targets/ftp.md), [rsync](../05-publishing/targets/rsync.md), [folder](../05-publishing/targets/folder.md), [command](../05-publishing/targets/command.md).
