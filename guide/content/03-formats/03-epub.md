# The EPUB

The EPUB is one file that reading apps open: Apple Books, Kobo, Calibre, Thorium, and most others. Octavo writes it by itself, with nothing installed.

## Made for reading apps

- **The reader stays in charge.** The book sets no font, text colour or page colour, and only relative sizes, so the app's own font, size, margins and night mode all work. Code colours switch to a dark set on dark pages.
- **Contents and navigation.** A table of contents with the parts and sections nested, and landmarks for the cover, title page, contents and the start of the book. An older-style table of contents (NCX) is included too, for EPUB 2 apps.
- **A cover.** Your own image, or one drawn from the title and author, because a book with no cover looks broken in an app's library.
- **Footnotes** are marked so that apps can show them as pop-ups.
- **Accessibility.** Chapters, parts and footnotes carry their roles for screen readers, and the file states its accessibility features.

The file follows EPUB 3.3 and is valid XML throughout, even when a chapter holds loose HTML.

## Settings

```yaml
format:
  epub:
    cover: images/cover.png     # your cover; "none" for no cover
    css: epub.css               # extra styles for the e-book
```

| Setting | Meaning | Default |
|---|---|---|
| `cover` | A PNG or JPEG cover image, or `none` | A cover drawn from the title |
| `output-file` | The file name | The title, such as `my-book.epub` |
| `css` | Your stylesheets, after the built-in one | |
| `highlight`, `highlight-dark` | Code colour styles for light and dark pages | `github`, `github-dark` |
| `number-chapters` | `false` for no chapter numbers | `true` |

> [!TIP]
> Keep an e-book stylesheet small. Reading apps let the reader choose fonts and colours, and an e-book that fights them is hard to read.
