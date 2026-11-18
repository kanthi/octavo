# Your first book

## Create it

```sh
octavo new my-book
cd my-book
```

`octavo new` makes a small sample book to start from:

```text
my-book/
├── octavo.yml          the book's settings and chapter list
├── index.md            the preface: the book's first page
├── chapters/
│   ├── 01-intro.md
│   └── 02-basics.md
├── images/cover.svg
├── custom.css          your own styles (all commented out at first)
└── .gitignore          keeps the built book out of git
```

`octavo new` never overwrites a file. It refuses a folder that already holds a book, and a folder inside a [library](../04-libraries/01-libraries.md), where books are added with `octavo add` instead.

## The settings file

`octavo.yml` names the book and lists its chapters in reading order:

```yaml
book:
  title: My Book
  author: Your Name
  chapters:
    - index.md
    - chapters/01-intro.md
    - chapters/02-basics.md
```

Every setting is described in [Book settings](../06-reference/02-book-settings.md).

## Write

Chapters are plain Markdown. Edit `chapters/01-intro.md`, or add a file and list it under `chapters:`. The [Writing](../02-writing/01-chapters.md) part of this guide covers what you can put in a chapter.

## See it

```sh
octavo preview --open
```

This opens the book in your browser and updates it every time you save. See [Live preview](03-preview.md).

## Build it

```sh
octavo render
```

The finished book is in `_book/`: the website (open `_book/index.html`), plus `my-book.epub` and `my-book.pdf`. To build only the website, which is much faster:

```sh
octavo render --to html
```

## Publish it

Add a target to `octavo.yml`, then run `octavo publish`. For GitHub Pages, from inside a git repository with a GitHub remote:

```yaml
publish:
  gh-pages: {}
```

```sh
octavo publish
```

[Publishing](../05-publishing/01-publishing.md) lists every kind of target.
