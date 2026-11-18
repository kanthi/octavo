# Libraries

A library is a folder of several books that are built and published together, as one site. A portal page lists every book as a card.

```text
my-library/
├── octavo-library.yml    the library's settings: title, folders, publish targets
├── books/
│   ├── Linux/            an ordinary book: octavo.yml, chapters, its own _book/
│   └── Go/
└── public/               the site to host
    ├── index.html        the portal
    ├── html/Linux/       each book's website
    ├── pdf/Linux.pdf     each book's PDF
    └── epub/Linux.epub   and EPUB
```

## Start a library

```sh
octavo new --library my-library --books Linux,Go
```

This makes the library and two books in it. `--books` is optional; see [Adding books](02-adding-books.md) for adding them later, or for bringing in a book you already have.

## The same commands, for every book

Inside a library folder, or with `--dir` pointing at one, the usual commands work on all of its books:

```sh
octavo render                  # every book, then the portal
octavo render Linux Go         # only these two; the portal keeps the others
octavo preview --open          # the portal and every book, with live reload
octavo publish                 # build everything, then send public/
```

Each book is built into its own `books/<Name>/_book/` and then copied into `public/`. Book names on the command line match in any letter case.

A book that fails to build does not stop the others. The run reports it, carries on, and exits with an error at the end.

## The portal

`public/index.html` has a card for every book: its title, author, description and number of chapters, with links to read it online or download the PDF and EPUB. Each card has a colour of its own. A search box filters the cards, and a button switches between light and dark.

Every page of every book links back to the portal from its top bar.

[Drafts](../02-writing/04-drafts.md) are left off the portal, unless built with `--drafts`; they then carry a **Draft** badge.

## Library settings

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

Every key is described in [Library settings](../06-reference/03-library-settings.md). Publish targets for a library go here, not in the books: `octavo publish` sends the whole `public/` folder.
