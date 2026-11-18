# Live preview

```sh
octavo preview          # or: octavo dev
octavo preview --open   # and open it in your browser
```

The preview serves the book at `http://127.0.0.1:4000` and watches the book's folder. Each time you save a file, it rebuilds what the change affects and reloads the browser tabs that show it. Stop it with <kbd>Ctrl</kbd>+<kbd>C</kbd>.

## Only what changed is rebuilt

| You change | What is rebuilt |
|---|---|
| The text of a chapter | That chapter's page and the search index |
| A chapter's title, or the chapter list | Every page of the book, because every sidebar shows them |
| A stylesheet or an image | That one file |
| A book setting, or the page template | Every page of the book |
| One book of a [library](../04-libraries/01-libraries.md) | Only that book, and the portal if its card changed |

Octavo does not guess from file names. It remembers a fingerprint of everything each page was made from, and rewrites a page only when its fingerprint changes. A chapter edit in a large book takes a few milliseconds.

## What the preview does not do

- **It builds the website only.** Printing a PDF on every save would be far too slow. Run `octavo render` for the EPUB and PDF.
- **It writes to a temporary folder,** not to `_book/`. `octavo render` is always a full, clean build into `_book/`.
- **It leaves out drafts** unless you start it with `--drafts`. See [Drafts](../02-writing/04-drafts.md).

## Options

| Flag | Meaning |
|---|---|
| `--dir <path>` | The book or library folder, if not the current one |
| `--port <number>` | Another port than 4000 |
| `--open` | Open the browser |
| `--drafts` | Include books marked as drafts |
