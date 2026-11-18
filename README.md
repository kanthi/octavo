# Octavo

Write books in Markdown. Octavo builds them as a website, an EPUB and a PDF, and publishes them. It is one program written in Go, modelled on the book workflow of [Quarto](https://quarto.org) with far fewer features, and not affiliated with it.

Version `0.1.0`. Documentation: **[www.kanthi.in/octavo](https://www.kanthi.in/octavo/)**, also built into the program as `octavo guide`.

## What you get

- **A website** for each book: chapter list, search, light and dark pages, reading settings (font, size, spacing, width), zen mode, and copy buttons on code. Readers can save the book to read offline. Fonts are bundled, so nothing loads from other sites.
- **An EPUB** that opens in Apple Books, Kobo, Calibre and older EPUB 2 apps.
- **A PDF** laid out as a printed book: title page, contents with page numbers, running headers, footnotes at the foot of the page. It is set by [Typst](https://typst.app), or printed by Chrome when Typst is not installed.
- **Libraries:** many books on one site, with a portal page that gives each book a card in its own colour.
- **Publishing** to GitHub Pages (from your machine, or by GitHub Actions on every push), FTP, rsync, a folder, or any upload command.
- **Books that grow easily:**
  - a new numbered chapter file (`06-sensors.md`) joins the book by itself;
  - a moved chapter's old address forwards to its new one;
  - older editions can stay online beside the latest, with a version menu.

## Install

With Go 1.27 or later:

```sh
go install github.com/kanthi/octavo/cmd/octavo@latest
```

Or from the source:

```sh
git clone https://github.com/kanthi/octavo.git
cd octavo
go build -o octavo ./cmd/octavo
```

Websites and EPUBs need nothing else. PDFs need [Typst](https://typst.app) (`brew install typst`), or Chrome, Chromium, Edge or Brave.

## Start

```sh
octavo new my-book          # a sample book to start from
cd my-book
octavo preview --open       # see it in your browser, updated on every save
octavo render               # build the website, EPUB and PDF into _book/
octavo check                # broken links, missing images, files left out
octavo publish              # send it to a host, once one is set in octavo.yml
```

Several books on one site, with a portal page:

```sh
octavo new --library my-library --books Linux,Go
cd my-library
octavo add Rust             # more books later
octavo preview --open
```

Publish from GitHub on every push:

```sh
octavo publish --setup github   # writes .github/workflows/octavo.yml
```

## Learn more

```sh
octavo guide
```

The Octavo Guide opens in your browser: every feature, command and setting. It is built into the program, so it always matches your version. `octavo guide --to pdf` saves it as a PDF.

## Working on Octavo

```sh
go test ./...
octavo preview --dir guide            # the guide's source is an Octavo book in guide/
octavo preview --dir examples/library # a sample library of five books
```

Design notes and decisions: [docs/product.md](docs/product.md). Plans: [docs/todo.md](docs/todo.md). History: [AGENT_WORKLOG.md](AGENT_WORKLOG.md).

Every push to `main` tests Octavo and publishes the guide on GitHub Pages ([.github/workflows/docs.yml](.github/workflows/docs.yml)).

## Licence

Octavo is under the [MIT License](LICENSE).

It bundles the fonts Literata, Source Serif 4, Atkinson Hyperlegible Next and JetBrains Mono, which are under the SIL Open Font License; each has its licence beside it in [internal/format/fonts/files](internal/format/fonts/files). Portions of this software are copyright © The FreeType Project (www.freetype.org). All rights reserved.
