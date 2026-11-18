# Library settings

A library's settings are in `octavo-library.yml`, at the top of the library's folder. Each book keeps its own `octavo.yml` as usual.

```yaml
library:
  title: Our Books
  description: Notes and guides, as books.
  books-dir: books
  output-dir: public
  theme: auto
  css: portal.css

publish:
  gh-pages: {}
```

## library

| Key | Meaning | Default |
|---|---|---|
| `title` | The portal's title, and the label of the link back to it on every book page | `Books` |
| `description` | A sentence under the portal's title | |
| `books-dir` | The folder holding one subfolder per book. Every subfolder with an `octavo.yml` is a book. | `books` |
| `output-dir` | The site to host: the portal plus every book | `public` |
| `theme` | The portal's `light`, `dark` or `auto` colours | `auto` |
| `css` | Your stylesheets for the portal page | |

## publish

The same targets as for a book; see [Publishing](../05-publishing/01-publishing.md). They send the whole `output-dir`. A book's own `publish:` section is not used inside a library.

## Styling the portal

A portal stylesheet can set these variables:

```css
:root {
  --font-title: "Iowan Old Style", Georgia, serif;   /* the book titles and the heading */
  --font-ui: system-ui, sans-serif;
  --bg: #f6f5f2;
  --accent: #2456c9;
}
```

Each card's colours are tints of one hue, chosen so that no two books look alike.
