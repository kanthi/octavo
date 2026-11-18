# The website

The website is a folder of plain web pages with relative links. It works from any folder on any web server, and from your own disk.

Every page has the chapter list on the left, the chapter in the middle, and "On this page" on the right. The bar at the top holds the book's title, a search box, and the reader's controls.

## What readers can change

These controls are for the person reading. Their choices are remembered in their browser, for every book on the same site.

| Control | What it does |
|---|---|
| <kbd>☰</kbd> | Hide or show the chapter list. On a phone it opens the list over the page. |
| The chevron button at the top of the chapter list | In books with parts: expand all or collapse all. Only the part you are reading is open at first. |
| The small panel button at the top right | Hide or show "On this page". |
| **Aa** | Font, text size, line spacing, page width, and which side panels to show. "Reset" goes back to the book's own settings. |
| Zen button, or <kbd>Z</kbd> | **Zen mode:** hides the chapter list and "On this page" and fades the top bar until you point at it. The text then fills the window: the type grows with the screen, keeping the same number of words per line. The **Aa** panel's Width picks how many; a smaller text size fits more on each line. <kbd>Z</kbd> or <kbd>Esc</kbd> leaves it. |
| <kbd>◐</kbd> | Light or dark pages. |

The **Aa** panel offers three fonts made for long reading, bundled with every book so nothing loads from another site:

- **Literata**, the default: a serif designed for reading on screens.
- **Source Serif 4**: a calmer, more neutral serif.
- **Atkinson Hyperlegible Next**: a sans-serif whose letters are hard to mistake for one another.

Code is set in **JetBrains Mono**, a font made for code: `0` and `O`, `1`, `l` and `I` are easy to tell apart. It is used in the PDF too, without programming ligatures, so `!=` and `->` look as typed.

All four are under the SIL Open Font License. Only Latin letters (and, for code, arrows and box-drawing lines) are bundled; other scripts use the reader's own fonts.

## Search

The search box looks through the whole book. It needs the book to be served by a web server, as the preview and every host do. Opened straight from disk (`file://`), search says so and does nothing.

## Reading offline

Readers can keep reading without a connection. Every page they open is kept in their browser, and **Save for offline reading**, under the chapter list, keeps the whole book at once, with its pictures. Pages still come fresh from the site whenever there is a connection, so nobody reads an old text by mistake.

It works on any site served over `https`, as every host serves it today, and on your own computer. It is off in the preview. To turn it off for a book:

```yaml
format:
  html:
    offline: false
```

## Moved chapters

When you move or rename a chapter, its old address keeps working: it forwards readers, and search engines, to the chapter's new place. So links from other sites, bookmarks and search results do not break.

Octavo keeps a list of every page address the website has had in `octavo-pages.txt`, beside `octavo.yml`. When an address from that list is no longer built, Octavo finds where its chapter went: the chapter with the same title, or else the one with the same file name. **Commit this file with the book**, so that a build on another computer, or by GitHub Actions, knows the old addresses too.

When a chapter is renamed *and* retitled, Octavo cannot tell, and says so once. Name the new place yourself; the same works for any address, including one on another site:

```yaml
format:
  html:
    redirects:
      chapters/old-name.html: chapters/new-name.md
      faq.html: chapters/help.md#questions
      blog.html: https://example.com/blog/
```

## Your own look

The settings under `format: html:` in `octavo.yml`:

```yaml
format:
  html:
    theme: auto            # light, dark, or auto (follows the reader's system)
    font: literata         # the starting font: literata, source-serif, atkinson, system
    css: custom.css        # your stylesheet, or a list of them
    page-toc: true         # false removes "On this page" from the whole book
```

### A stylesheet of your own

A stylesheet listed under `css:` loads after the built-in theme, so anything in it wins. The theme is built on CSS variables, so most changes are one line each:

```css
:root {
  --accent: #b5381f;              /* links and highlights */
  --content-width: 40rem;         /* the width of the text */
  --font-body: "Iowan Old Style", Georgia, serif;
}
[data-theme="dark"] {
  --accent: #ff8a6b;
}
```

| Variable | What it sets |
|---|---|
| `--font-body`, `--font-ui`, `--font-mono` | The text, the controls, and code |
| `--content-width`, `--sidebar-width` | The width of the text and of the chapter list |
| `--bg`, `--bg-soft`, `--text`, `--text-soft`, `--border`, `--accent` | The colours; set them again under `[data-theme="dark"]` for dark pages |

The `custom.css` that `octavo new` writes lists them all, commented out. This guide's own stylesheet is `styles/guide.css`.

> [!NOTE]
> A reader's font choice in the **Aa** panel wins over `--font-body`, which wins over `font:` in `octavo.yml`.

### Code colours

`highlight-light` and `highlight-dark` choose the colour style for code on light and dark pages. They take any [Chroma style](https://xyproto.github.io/splash/docs/) name; the defaults are `github` and `github-dark`.

### Other files

Images used in chapters are copied by themselves. For anything else the pages need, such as a font file your stylesheet loads, list it under `resources:`. Patterns such as `fonts/*.woff2` work.

### Your own page template

For full control, `template:` names an HTML file that replaces the built-in page template. Start from a copy of the built-in one, which is `internal/format/html/theme/page.html` in Octavo's source. A template is tied to the Octavo version it was copied from: new features, such as the reader's controls, appear only in the built-in one.
