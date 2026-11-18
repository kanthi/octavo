# Choosing formats

Octavo builds a book three ways:

| Format | Name | What you get |
|---|---|---|
| Website | `html` | A folder of web pages, in `_book/` |
| E-book | `epub` | One `.epub` file for reading apps such as Apple Books, Kobo or Calibre |
| PDF | `pdf` | One `.pdf` file, laid out like a printed book |

## Which formats are built

A plain `octavo render` builds the formats listed under `format:` in `octavo.yml`. A new book lists all three. A book with no `format:` section at all also gets all three.

```yaml
format:
  html:
    theme: auto
  epub: {}          # "{}" means: build it, with no settings of its own
  pdf:
    paper: A4
```

To leave one out, remove its line, or write `pdf: false`.

## Choosing for one run: `--to`

```sh
octavo render --to html           # only the website: the quick one while writing
octavo render --to pdf,epub       # several, separated by commas
octavo render --to all            # every format, whatever octavo.yml lists
```

`--to` works on `render` and `publish`. `--format` is another name for it. To set it once for a whole terminal session or a CI job, use the `OCTAVO_TO` environment variable:

```sh
export OCTAVO_TO=html
octavo render                     # now builds only the website
```

`--to` on the command line wins over `OCTAVO_TO`.

## Where the files go

Everything goes into `_book/`, or the folder named by `output-dir` in `octavo.yml`. The EPUB and PDF are named after the book's title: a book called "My Book" gives `my-book.epub` and `my-book.pdf`. `output-file` under `epub:` or `pdf:` chooses another name.

## Good to know

- **A partial build keeps the rest.** `octavo render --to pdf` adds the PDF and leaves the website in `_book/` as it is. A plain `octavo render`, and `--to all`, empty `_book/` first.
- **Download links.** When the website is built in the same run as the EPUB or PDF, the end of the sidebar links to them. `--to html` on its own leaves the links out.
- **The preview is always the website.** See [Live preview](../01-start/03-preview.md).
