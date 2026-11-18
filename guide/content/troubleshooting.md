---
title: Troubleshooting
unnumbered: true
---

## "pdf skipped: PDF needs Typst or a Chrome-family browser"

Octavo needs Typst, or Chrome, Chromium, Edge or Brave, to make a PDF. Install Typst (`brew install typst`) or a browser, or point Octavo at one with `typst:` or `browser:` under `format: pdf:`. See [The PDF](03-formats/04-pdf.md#two-engines-typst-and-a-browser).

## "… not added to the book: its name has no number"

New chapter files join the book by the number at the start of their name. This one has none, so Octavo cannot tell where it goes. Rename it, such as `07-name.md`, or list it in `octavo.yml`. To keep it out of the book quietly, put `_` in front of its name. See [New chapters join by their number](02-writing/01-chapters.md#new-chapters-join-by-their-number).

## A new chapter does not appear

- Its name needs a number, and it must sit next to the book's other chapters, or in a numbered folder beside theirs. `octavo check` lists the chapter files that are left out, and why.
- With `auto-add: false`, only the files listed in `octavo.yml` are in the book.
- A site served by a plain web server shows the last `octavo render`, not your newest files. Render again, or use `octavo preview`, which rebuilds on every save.

## Search says it needs a web server

Search loads its index over the network, which browsers do not allow for pages opened straight from disk. Use `octavo preview`, or any web server, such as `python3 -m http.server --directory _book`.

## "… is inside the library …; add books to it with octavo add"

`octavo new` is only for starting out. Inside a library, use `octavo add <Name>`. See [Adding books](04-libraries/02-adding-books.md).

## "… is a draft; add --drafts to build it"

The book has `draft: true` in its `octavo.yml`. Add `--drafts`, or remove the line when the book is ready. See [Drafts](02-writing/04-drafts.md).

## A link between chapters does not work

Link to the chapter's `.md` file, relative to the chapter you are writing, and make sure that file is in the book: listed under `chapters:`, or added by its number. `octavo check` finds broken links.

## The chapter list, or "On this page", has disappeared

You, or someone on this browser, hid it. Press <kbd>Z</kbd> to leave zen mode, or use <kbd>☰</kbd> and the **Aa** panel's Layout buttons. "Reset to the book's settings" in the **Aa** panel brings everything back.

## The preview does not show a change

The preview watches the folder of the book or library it was started in. Files outside it, such as a stylesheet listed with `../`, are not watched; save a file inside the book to rebuild.

## Something else

Run the command again with `-h` to check its flags, and see the [Reference](06-reference/01-commands.md).
