# Octavo agent worklog

## 2026-11-14: project created

**Request:** "add new project called octavo (golang version of quarto like tool with minimal features) for creating and publishing books". The user said earlier: do not commit anything; remind them when a commit seems necessary.

**Done**
- Scaffolded `apps/octavo/` per the incubator rules: README, `docs/product.md` (book layout, `octavo.yml`, commands, first-version features, later, out of scope, open questions), Go `.gitignore` (binary, `dist/`, rendered `_book/`, secrets), `.env.example`, this worklog.
- Root README catalog row added.

**Decisions (mine, open to change)**
- Minimal scope: HTML book + EPUB, `new` / `render` / `preview` / `publish gh-pages`. No code execution, which is Quarto's main feature but out of scope here.
- Planned libraries: goldmark (Markdown), chroma (highlighting), `html/template`, `embed`. PDF only later, through Typst or Pandoc on PATH.

## 2026-11-15: first working version (HTML, preview, publish)

**Request:** the user answered the open questions: custom CSS themes yes; HTML first, EPUB and PDF required later; GitHub Pages is one host among several, self-hosting must be easy; deploy options like MkDocs (gh-pages, FTP now, maybe Cloudflare); make the app as modular as possible.

**Done**
- `go mod init octavo` (Go 1.27.4). Dependencies: goldmark, goldmark-highlighting, chroma, yaml.v3, jlaffaye/ftp.
- Commands: `new`, `render`, `preview`, `publish [target]`, `version` (`internal/cli`, standard `flag` package, no CLI framework).
- Modular layout: `format.Format` and `publish.Publisher` interfaces, each with a registry. Every module is registered in `internal/cli/modules.go` and decodes its own YAML node from `octavo.yml`, so `internal/config` never changes when a module is added.
- `internal/markdown` parses to a format-neutral document (title, headings, images, plain text) so EPUB can reuse it. Callouts are a custom goldmark node.
- HTML format: built-in theme driven by CSS variables, user `css:` loaded after it, optional `template:` override, `resources:` globs, light/dark/auto, search index, copy buttons. All links are relative, so the site works in any subfolder.
- Publish modules: `gh-pages` (one force-pushed commit, adds `.nojekyll` and optional `CNAME`), `ftp` (explicit TLS by default), `rsync`, `folder`, `command`. A target's `type:` defaults to its name.
- Preview renders to a temp folder with live reload over server-sent events; file changes are found by polling every 400 ms (no fsnotify dependency).
- Docs updated: README, `docs/product.md`, `.env.example`, root README catalog row.

**Decisions (mine, open to change)**
- The leading `# Heading` of a chapter is removed from the body and used as the title; front matter `title` wins over it.
- A leading `index.md` is the unnumbered preface.
- `render` empties the output folder first, and refuses an output folder that contains the book.
- FTP and `folder` never delete remote files; `rsync` deletes only with `delete: true`.

**Status**
- `go build ./...`, `go vet ./...`, `go test ./...` pass. Tests cover Markdown parsing, a full `new` + `render` of the sample book, config errors, and publishing through `folder`, `command`, and `gh-pages` (against a local bare repository).
- Checked by hand: preview with live reload, light, dark and narrow layouts in headless Chrome, `rsync` to a local folder.
- **Not tested:** the `ftp` module has never run against a real FTP server. `gh-pages` has not been pushed to GitHub itself.
- Nothing committed (the user asked not to commit; remind them when it makes sense).

**Next steps**
1. EPUB format module (`internal/format/epub`), reusing `markdown.Document`.
2. PDF format module through Typst or Pandoc on `PATH`.
3. Try `ftp` against a real server; decide whether it should delete stale files.
4. Dedicated Cloudflare Pages module if `command` is not enough.
5. Pick a real Go module path before sharing.

## 2026-11-16: libraries (multi-book portal), parts and sections, sample books

**Request:** test with several small sample books like the user's Quarto books repo (Linux, Go, Rust, ...), render and open them. Each book renders to its own build folder and is also copied to a public folder where a portal hosts all books, as in that repo.

**Reference:** the user's repo is `~/Workspace/Repos/books` (Quarto). There, `books/<Name>/_quarto.yml` builds to `books/<Name>/_book`; `books/renderpub.sh` builds every book and assembles `books/published_books/` (`index.html` portal, `html/<Name>/`, `pdf/`, `epub/`, `assets/` covers); `books/indipub.sh <Name>` builds one. Books use `part:` / `section:` / `text:` + `file:` in the chapter list.

**Done**
- `internal/library`: `octavo-library.yml` marks a library. `render`, `preview`, `publish` detect it (no new commands). Full build empties `public/`, builds each book into its own `_book`, copies to `public/html/<Name>/`, writes the portal `public/index.html` (single self-contained file: cards, search, theme switch, hue per book name). `octavo render <Name>...` builds only those and keeps the rest. Failed books are skipped and reported; exit code is 1.
- `octavo new --library <dir>`.
- Chapter list entries: plain path, `{text, file}`, `{part|section, contents}` nested to any depth (`config.Entry`, `book.Node`). Sidebar renders groups as `<details>`; top-level always open, nested open only around the current page.
- `format.Options.HomeURL/HomeLabel`: a link from every book page back to the portal. Set only by library builds.
- `preview.Run` now takes a build function, shared by books and libraries. Library preview builds straight into the temp folder.
- `examples/library/`: four sample books (Linux with nested sections, Go, Rust, NixOS), each with `content/NN-part/` folders and `styles/custom.css`.
- Tests: `TestLibrary`, `TestPartsAndSections`.

**Decisions (mine, open to change)**
- A library book's `_book` carries the portal link (`../../index.html`), so it only resolves from inside `public/`. Rendering that book alone with `--dir books/<Name>` leaves the link out.
- Portal covers are CSS gradients with the title's initials, not generated SVG files.

**Status**
- `go build`, `go vet`, `go test ./...` pass. Sample library renders 4 of 4 books; portal, nested sidebar, home link and per-book CSS checked in headless Chrome. `publish` of the library checked with the `folder` target.
- Still not tested: `ftp` against a real server, `gh-pages` against GitHub.
- Nothing committed.

**Next steps**
1. EPUB, then PDF. When they exist, copy them to `public/epub/<Name>.epub` and `public/pdf/<Name>.pdf` and add links on the portal cards.
2. Try Octavo on one real book from `~/Workspace/Repos/books` (needs `.qmd` handling and a `_quarto.yml` to `octavo.yml` conversion; not started).
3. Carried over: real FTP test, Cloudflare module, Go module path.

## 2026-11-16: multi-level sample book

**Request:** create a multi-level book as an example to test deep structure.

**Done**
- `examples/library/books/Networking/`: 12 chapters, chapter list four levels deep (part > section > sub-section > chapter), files nested up to five folders deep, section intro pages (`index.md` inside a section), a top-level chapter beside the parts (unnumbered glossary via front matter), an image referenced from a nested chapter (`../../images/stack.svg`), cross-links between chapters at different depths, and h2/h3 headings for the "on this page" list.
- No code changes were needed.

**Status**
- Renders 5 of 5 books. A link check over every `href` and `src` in `public/` found 490 links, 0 broken. Sidebar nesting, collapse state and the image checked in headless Chrome.
- Nothing committed.

## 2026-11-17: incremental rebuilds in the live preview

**Request:** a live dev runner that watches for file changes and re-renders only the parts that need it, deciding dynamically.

**Done**
- `internal/format/output.go`: `format.Output` is the render target. It remembers a fingerprint per output file; `Fresh(rel, fingerprint)` lets a format skip a file whose inputs did not change, and files a build no longer asks for are deleted. A new `Output` has no memory, so it does a clean full build.
- `Format.Render` now takes `*format.Output` instead of a folder path (interface change; one implementation).
- HTML format: a page's fingerprint is its chapter's source hash plus a "shell" hash of everything else the page shows (book settings, HTML settings, template source, and every chapter's path, title and number). So a text edit redoes one page; a title or chapter-list change redoes all pages of that book. Assets and `search.json` are written only when their bytes or source file change.
- `book.Cache` / `book.LoadCached`: parsed chapters are reused when a file's modification time and size are unchanged. `Document.PlainText` is memoised.
- `library.State`: per-book `Output` and `Cache` kept between builds; the portal is rewritten only when its bytes change.
- `preview.Run`: the build function returns the changed files. The log line lists them with the time taken. The reload event carries the list; a tab reloads only if its own page, or a non-HTML file, changed.
- `octavo dev` is an alias of `octavo preview`.
- Tests: `TestIncrementalBook`, `TestIncrementalLibrary` assert the exact set of rebuilt files for each kind of edit.

**Decisions (mine, open to change)**
- `octavo render` stays a clean full build; only preview is incremental (state lives in memory, nothing is written to disk between runs).
- Change detection is still a poll of the source tree every 400 ms (stat only).

**Status**
- `go build`, `go vet`, `go test ./...` pass. Tried live on `examples/library` (5 books, 57 files): first build 40 ms; a chapter text edit rebuilt 2 files in 4 ms; a title edit rebuilt that book's 7 files; a CSS edit 1 file; touching an unrelated file rebuilt nothing.
- Known limits: a changed stylesheet or image reloads every open tab, including tabs on other books. A full page reload is used; styles are not swapped in place. Not measured on a large book (the user's Linux book has about 1000 chapters).
- Nothing committed.

## 2026-11-16: EPUB and PDF output, `--to` to choose formats

**Request:** add PDF and EPUB render options; the user can pass `all`, `html`, `pdf`, `epub` to choose what to render.

**Done**
- `--to <list>` on `render` and `publish` (`--format` is another name): `all` or a comma-separated list. `format.Select` parses it; `format.Options.Formats` carries it. Without it, the formats under `format:` in `octavo.yml` are built, as before.
- `internal/format/epub`: EPUB 3 written with `archive/zip`, no external tool. Title page, optional `cover:`, nested `nav.xhtml` from parts and sections, images, one stylesheet. File: `_book/<title-as-slug>.epub`.
- `internal/format/pdf`: writes the whole book as one print-ready HTML page (title page, contents, part pages, chapters) and has a headless Chrome-family browser print it. `FindBrowser` looks at `browser:`, `$OCTAVO_BROWSER`, `PATH`, then the usual macOS and Windows install folders. `command:` swaps in any HTML-to-PDF tool (`{input}`, `{output}`). File: `_book/<title-as-slug>.pdf`.
- `internal/format/xhtml`: shared by both. Renders a chapter to HTML, parses it like a browser (`golang.org/x/net/html`, new dependency) and writes it back as well-formed XML, so raw HTML in chapters cannot break an EPUB. Also the content stylesheet, link and image resolution, output file name.
- Library: `public/pdf/<Name>.pdf` and `public/epub/<Name>.epub`; portal cards get Read / PDF / EPUB links. Card markup changed from one `<a class="card">` to a `<div class="card">` holding `<a class="open">` and the links.
- Starter `octavo.yml` has commented `epub:` and `pdf:` sections.
- Tests: `TestFormats`, `TestPDF` (skipped when no browser is installed), `TestLibraryFormats`.

**Decisions (mine, open to change)**
- PDF through a browser, not Typst or Pandoc (neither is installed here; a browser reuses the chapter HTML, so nothing needs a second renderer). This closes the open question in `docs/product.md`.
- `--to` with only some formats does not empty the output folder, so `--to pdf` keeps the website. A plain `render` and `--to all` are clean builds. Side effect: a partial build can leave stale files behind until the next full build.
- `octavo preview` always builds HTML only, even when `octavo.yml` enables PDF.
- EPUB and PDF use fixed, light styling and their own `css:` lists, not the HTML theme.
- In PDF, element ids get a chapter prefix (`ch3-setup`) because all chapters share one page.

**Status**
- `go build`, `go vet`, `go test ./...` pass, including `TestPDF` with Chrome on this Mac.
- Tried on `examples/library` with `--to all`: 5 of 5 books, about 4.5 s. Networking PDF: 16 pages, 15 bookmarks, page numbers, SVG image, parts and nested contents checked by eye. EPUB pages pass `xmllint`. Portal links checked in headless Chrome.
- **Not done:** `epubcheck` is not installed, so the EPUB was never formally validated, and it was not opened in a reading app. PDF was only tried with Chrome on macOS; Edge, Brave, Linux, Windows and the `command:` route are untested. Not tried on a large book.
- Known limits: the PDF contents page has no page numbers (the browser cannot fill them in); remote images in an EPUB are left as web links; the HTML book has no download links to its own PDF and EPUB.
- Nothing committed.

**Next steps**
1. Validate an EPUB with `epubcheck` and open one in Apple Books.
2. Download links on the HTML book pages.
3. Carried over: a real book from `~/Workspace/Repos/books`, real FTP test, Cloudflare module, Go module path.

## 2026-11-16: download links on the HTML book

**Request:** "get back to octavo" after the EPUB and PDF work. I took the two gaps from the entry above: download links, and validating the EPUB.

**Done**
- The HTML book's sidebar ends with "Download EPUB / PDF" links when those formats are built in the same run.
- How: formats that write one file implement `format.Filer` (`File(b)` returns the file name). `RenderAll` collects them into `Options.Downloads` before any format runs, so HTML knows the names even though PDF is rendered after it. `Options.DownloadURL` lets a library point the links at `public/pdf/` and `public/epub/`. The links are part of the page fingerprint.
- Tests extended in `TestFormats` and `TestLibraryFormats`.

**Status**
- `go build`, `go vet`, `go test ./...` pass. Links checked in headless Chrome on the sample library.
- Known limit: the links follow the run, not the folder. `--to html` on its own rebuilds pages without links even if a PDF from an earlier run is still in `_book/`; `--to pdf` on its own does not add links to existing pages.
- **EPUB still not validated:** `epubcheck` needs Java, and this Mac has no Java runtime. I did not install one. To check: `brew install epubcheck`, then `epubcheck _book/<name>.epub`.
- Nothing committed.

## 2026-11-16: first run on real books (`~/Workspace/RnD/octavo-test`)

**Request:** create a new library in `~/Workspace/RnD/octavo-test`, copy a few books from the Quarto books repo (Networking, NixOS, Go) as Markdown, render, and open the portal. (The user wrote "using quarto"; I read it as Octavo, given the folder name.)

**Done**
- `~/Workspace/RnD/octavo-test`: a Octavo library made with `octavo new --library`, outside this repo. It has its own README.
- `tools/import-quarto.rb` there converts a Quarto book: `_quarto.yml` to `octavo.yml`, `.qmd` to `.md`, callout blocks to `> [!NOTE]`, `content-visible` blocks, `{{< include >}}`, heading and image attributes, `.qmd` links. Ruby, because this Mac's Python has no YAML module and the system Ruby does.
- Imported Go (425 chapters), Networking (170), NixOS (199).
- No Octavo code changes were needed.

**Results (this answers "not tried on a large book")**
- `octavo render`: 794 chapters in about 4 seconds, no warnings.
- `octavo render --to all`: about 2 minutes 10 seconds, nearly all of it Chrome printing. PDFs: Go 2561 pages (73 MB), NixOS 1291 (34 MB), Networking 915 (22 MB), all with bookmarks. EPUBs: 3.7 MB, 1.1 MB, 0.8 MB.
- Link check over the built site: 257,116 local links, 0 broken.

**Noticed, not fixed**
- The NixOS source lists three `_partial` files both as includes and as chapters; the importer drops them from the chapter list.
- PDFs are large (about 28 KB per page). Not investigated; likely the embedded fonts and SVG diagrams.
- Dark-mode diagram variants (`*-dark.svg`) are not used; Octavo has no light/dark image switch.
- The importer lives in the test library, not in Octavo. A `octavo import` command would be the natural home if this is kept.
- EPUB still not validated with `epubcheck` (no Java here).
- Nothing committed.

## 2026-11-16: reading fonts and the "Aa" reading-settings menu

**Request:** which fonts are best for long reading (top three), add them, and is it a good idea to let the reader change fonts and the like next to the dark/light switch, as EPUB readers do.

**Done**
- Three bundled fonts, chosen by me: Literata (new default), Source Serif 4, Atkinson Hyperlegible Next. Variable WOFF2, Latin and Latin Extended, normal and italic: 12 files, about 490 KB, in `internal/format/html/theme/fonts/` with the three OFL licence texts. Source: the `@fontsource-variable/*` packages (5.3.x) on jsDelivr. Every book gets them in `assets/fonts/`.
- `book.css`: `@font-face` rules; `--font-literata`, `--font-source-serif`, `--font-atkinson`, `--font-system`; `--text-scale`, `--line-height`. Sizes inside `article` changed from rem to em so the text size setting scales headings, tables and footnotes together.
- `page.html`: an "Aa" button and panel in the top bar (font, text size, line spacing, page width, reset). The inline head script applies saved settings before first paint. `book.js` handles the panel; it does nothing when a custom template has no panel.
- Storage: `localStorage["octavo-reader"]`, a JSON object holding only what differs from the book's defaults.
- `format: html: font:` in `octavo.yml` sets the book's default (`literata`, `source-serif`, `atkinson`, `system`).
- Precedence: reader's choice (`html[data-font]`) beats the author's `--font-body` in a custom stylesheet, which beats `font:` in `octavo.yml`.
- Tests: file counts in `incremental_test.go` went from 9 to 24 per book (15 font files).

**Status**
- `go build`, `go vet`, `go test ./...` pass. Checked in headless Chrome on the real Networking book: the panel layout, and Literata, Source Serif and Atkinson rendering as the default font.
- **Not checked:** clicking through the panel in a real browser (the screenshots forced the panel open; the click handlers were not exercised), the narrow-screen layout of the panel, and Safari and Firefox.
- Known limits: with a custom `--font-body`, the panel still highlights the `octavo.yml` font as current. A page template written before this change has no "Aa" button and no `data-default-font`. PDF and EPUB do not use these fonts. The portal page has no menu.
- `~/Workspace/RnD/octavo-test` was rebuilt with `--to html`, so its pages currently lack the PDF and EPUB download links until the next `--to all`.
- Nothing committed.

## 2026-11-16: PDF and EPUB by default, both reworked for reading

**Request:** build PDF and EPUB by default, and they must be properly formatted for reading, not just converted HTML; follow the best standards for both.

**Blocked, and what I did instead**
- My plan for PDF was a real typesetting engine (Typst), and `epubcheck` to validate the EPUB. `brew install typst` and `brew install epubcheck` were both refused by the session's permission settings. I did not work around that. So: the PDF still goes through the browser, with a proper book layout; the EPUB is checked only for well-formed XML. The Typst engine is written up under "Next" in `docs/product.md`.

**Done: defaults**
- A book with no `format:` section builds every format (`config.Load` no longer defaults to HTML; `format.RenderAll` does it). The starter `octavo.yml` lists `html`, `epub` and `pdf`.
- `format.Checker`: a format can say it cannot run here. PDF without a browser is skipped with a warning on a plain render, and is an error when asked for with `--to`.
- `OCTAVO_TO` is the default for `--to`. The tests set it to `html` in `TestMain`.

**Done: EPUB (`internal/format/epub`, rewritten)**
- Stylesheet sets no font, text colour or background and uses relative units; tints are translucent grey so they work in night mode. Code colours come in a light set and a dark set (`prefers-color-scheme`), with backgrounds stripped (`xhtml.HighlightColors`).
- Navigation document with nested contents and landmarks; `toc.ncx` and `<spine toc>` for EPUB 2; the contents page is in the reading order.
- A page for every part and section (any depth), so every contents entry has its own target.
- Generated SVG cover when no `cover:` is set; `cover: none` switches it off.
- `epub:type` and ARIA roles on chapters, parts, title page, footnotes (`noteref`, `footnotes`, `footnote`). Footnote ids have `:` replaced by `-`.
- Accessibility metadata. Only claims that hold for every book; hazard is `unknown`.
- Stable UUID (version 5 form) from title and author.

**Done: PDF (`internal/format/pdf`)**
- Per-chapter named pages (`@page ch12`) carry the book title and chapter title as running headers. Front matter is a named page with roman numerals.
- Bundled font via `@font-face` from files written beside the temporary HTML (`font:`, default Literata), `font-size`, `justify`, `headers` settings. Default margins `26mm 25mm 28mm`.
- `xhtml/content.css` is now the print stylesheet (PDF only): book-style tables, repeating table headers, wrapping code, callouts, footnotes.
- Fonts moved to `internal/format/fonts` (shared by HTML and PDF); HTML's `@font-face` rules are now generated from it.

**Status**
- `go build`, `go vet`, `go test ./...` pass.
- `~/Workspace/RnD/octavo-test`, plain `octavo render`: 3 of 3 books, 95 seconds. PDF pages: Go 2415, NixOS 1268, Networking 905. Pages of the Networking PDF checked by eye (title, contents, preface, part page, chapter opening, inner pages).
- EPUB: every XHTML, OPF, NCX and SVG file in the Networking EPUB passes `xmllint`. **Not validated with `epubcheck` and not opened in a reading app**, so claims of standards compliance are by construction, not by test.
- Known limits of the browser PDF: no page numbers on the contents page; the running header also appears on chapter-opening pages (Chrome does not apply `:first` to named pages); footnotes are at the end of the chapter; files are large (Go: 76 MB).
- The generated cover is SVG. EPUB 3 allows that; some older apps want PNG or JPEG covers and will show none.
- `examples/library` books still list only `html`.
- Nothing committed.

**Next steps**
1. With Typst available: `engine: typst` (see `docs/product.md`).
2. With Java available: run `epubcheck` on a built book and fix what it reports.

## 2026-11-17: renamed Guarto to Octavo, moved to its own repository

**Request:** the name Guarto reads as a Go port of Quarto. The user chose Octavo (the book format one size below a quarto), and asked for a new public GitHub repository, the project moved into it, and the rename applied everywhere. No commit yet: more work is planned before the first commit.

**Done**
- Public repository `kanthi/octavo` created with `gh` and cloned to `~/Workspace/Repos/octavo`. It has no commits.
- Project copied from `my-vibe-apps-mono/apps/guarto` (140 files; the built binary, `_book/` and `public/` were left behind as build output), then removed from the incubator. `apps/guarto/` there now holds only a pointer README, and the catalog row says `graduated`. The incubator's tracked `icon/` files show as uncommitted deletions.
- Rename, keeping case: `guarto` to `octavo` in every file, including earlier entries of this worklog. That covers the binary and `cmd/octavo`, `octavo.yml`, `octavo-library.yml`, the `OCTAVO_TO`, `OCTAVO_BROWSER` and `OCTAVO_FTP_PASSWORD` variables, the `.octavo/` ignore entry, `localStorage["octavo-reader"]`, the icon's label, and the sample library.
- Go module path is now `github.com/kanthi/octavo` (was the bare `guarto`). This closes the "pick a real Go module path" item.
- The test library moved from `~/Workspace/RnD/guarto-test` to `~/Workspace/RnD/octavo-test`, with its config files, README and importer renamed the same way.

**Status**
- `go build`, `go vet`, `go test ./...` pass in the new repository. `examples/library` renders 5 of 5 books and `octavo-test` renders 3 of 3 (HTML only; PDF and EPUB were not rebuilt after the rename).
- Nothing committed or pushed. The GitHub repository is public and empty.

**Before the first commit**
- This worklog and the README mention local paths and machine details (`~/Workspace/...`, "this Mac"). Decide whether the worklog belongs in a public repository, and trim it if so.
- No `LICENSE` file yet. The bundled fonts carry their own OFL texts.
- Readers who saved reading settings under the old `guarto-reader` key lose them; nothing migrates the key.

## 2026-11-17: version 0.007

**Request:** name this version 0.007, the first commit; do not commit yet.

**Done**
- `cli.Version` is now `"0.007"` (was `"0.1.0-dev"`); `octavo version` prints `octavo 0.007`. README status line updated.
- `go build`, `go vet`, `go test ./...` pass. Nothing committed.

## 2026-11-17: `add` for books, drafts

**Request:** `new` only for first-time creation; later books come with `add <bookname>`; never overwrite or disturb existing books. Cover three paths: single book to library, library from the start, adding books later. Books render one at a time or all together; books marked as drafts are skipped and need `--drafts`. No chapter-level drafts for now.

**Done**
- `octavo new` refuses to run anywhere inside a library (it walks up looking for `octavo-library.yml`) and points to `add`. `new --library <dir> --books A,B` creates a library with books in it. All names are checked before anything is created.
- `octavo add <Name>...` (`--dir` = library): creates books under `books-dir`. Refuses a name already in use (in any letter case, or any existing file or folder of that name), a name used twice, or a name that is not `[A-Za-z0-9._-]` starting with a letter or digit. A refusal creates nothing. A book made by `add` has no `publish:` section and no `.gitignore` (the library has both).
- `octavo add --from <book> [<Name>]`: copies an existing book in (`scaffold.CopyBook`), leaving the source as it is. Skips the book's `output-dir`, `.git`, `.hg`, `.svn` and `.octavo`; keeps symlinks as links. Refuses when the target would be inside the source. Notes when the copied book has a `publish:` section (ignored in a library) or is a draft. `add` in a single-book folder explains this route. This is the "single book to library" path; nothing is ever moved.
- Drafts: `book: draft: true`. Without `--drafts` (on `render`, `preview`, `publish`): skipped in a full library build with a log line and a count; a named draft is an error; a single book that is a draft is an error; drafts are kept off the portal; any build without `--drafts` deletes a draft's copy from `public/` (html, pdf, epub). With `--drafts`: built, with a "Draft" badge on the portal card. A library where every book is a draft fails with "nothing to build".
- `library.Open` (no books needed) beside `library.Load`; `Library.Draft`, `Library.Find`, `Library.Rel`; `Result.Drafts`.
- **Fixed while testing (older bugs):** `$OCTAVO_TO` never worked, because registering the `--format` alias with an empty default reset the shared variable. Every test therefore built PDFs and EPUBs; the suite went from about 17 s to 2.6 s. Fixing it exposed a second bug: with `--to`, an unknown format under `format:` in `octavo.yml` was silently ignored; it is now always an error.
- Tests: `library_test.go` with `TestNewLibraryWithBooks`, `TestNewOnlyForStartingOut`, `TestAddNeverTouchesExistingBooks`, `TestAddFromExistingBook` (compares a full snapshot of the source before and after), `TestDrafts`. Older library tests now use `add`.

**Status**
- `go build`, `go vet`, `go test ./...` pass. Also run by hand in a scratch folder: all three paths, a refused duplicate, a skipped draft, a refused named draft, and `preview --drafts` showing the badge.
- Nothing committed.

**Open**
- The `books:` portal-order list and `--skip` were proposed but not asked for; not built.

## 2026-11-17: portal cards redesigned

**Request:** redesign the portal cards: the title only once per card, in a good font; every card its own soft, pleasant colour.

**Done**
- Card: a tinted cover with the title (Literata 600, the only place the title appears) and the author in Literata italic, a darker "spine" stripe on the left, then the description, chapter count and Read / PDF / EPUB pills. The initials cover and the second title are gone. The portal heading is also Literata.
- Colours: each card has one hue (`--h`), and every colour on it is a soft tint of that hue, with a separate set for dark mode. `hues()` keeps the hue derived from the book's name unless it is too close to one already taken, then picks the free hue farthest from all others. So colours are distinct, and stay put as books are added unless they would clash.
- The Literata files (latin and latin-ext, normal and italic) are copied to `public/assets/fonts/`, written only when missing or changed. `--font-title` can be overridden in a library stylesheet.
- Test: `TestHuesAreDistinct` (1 to 100 books; adding a book keeps the others' colours).

**Status**
- `go build`, `go vet`, `go test ./...` pass. Screenshots of `examples/library` checked in light, dark and at 500 px wide.
- Nothing committed.

## 2026-11-17: `octavo guide`, short README, zen mode

**Request:** Octavo should contain a sample book that is its own help documentation; command `octavo guide`; README only install and quick start. Mid-task: an option to hide "On this page" and the left menu, a full-page reading (zen) mode.

**Done: the guide**
- `guide/`: the Octavo Guide, an ordinary Octavo book (25 chapters in six parts plus troubleshooting; a nested "Targets" section; callouts, tables, code, footnotes, an SVG diagram, raw HTML, its own stylesheet). `guide/guide.go` embeds it; `{{octavo-version}}` in Markdown and YAML is replaced with the running version.
- `octavo guide`: serves it on a free port and opens the browser (`--port`, `--no-open`). `--to pdf|epub|pdf,epub [--out dir]` saves `Octavo-Guide-<version>.<ext>`, refusing to overwrite. `--copy <dir>` writes the source as a book; the folder must not exist.
- Facts in the guide were checked against the code (settings structs, defaults, messages), not written from memory.
- Tests: `TestGuide` (renders a `--copy` with no warnings, version filled in, checks every local link and `#id`: about 300 links), `TestGuideDocumentsEverything` (every flag of every command, every format and publish target), `TestGuideSaveAs`. Both kinds of check were confirmed to fail on a deliberately broken flag doc and link.
- README cut to install, start, library start, `octavo guide`, and working on Octavo.

**Done: zen mode and layout**
- Top bar: a zen button (also the Z key; Esc leaves) hides the chapter list and "On this page" and fades the top bar until hovered. ☰ now also works on wide screens, collapsing the chapter list. The Aa panel has a Layout row: Chapters, On this page, Zen. All remembered in `octavo-reader` (`sidebar`, `toc`, `zen`) and applied before first paint. Reset clears them too.
- `format: html: page-toc: false` removes "On this page" for the whole book.
- `book.js`: one shared settings object for the Aa panel and the layout (previously the panel alone owned it).
- Layout grid now uses `--side-col` / `--toc-col` with fixed grid columns, so hiding a panel never shifts the text into the wrong column.

**Fixed along the way**
- The starter `custom.css` nested a `/* */` comment inside another, which ended the outer comment early and made its "commented-out" dark-mode block live CSS.

**Status**
- `go build`, `go vet`, `go test ./...` pass. Guide pages checked in headless Chrome, normal and in zen mode.
- **Not checked:** the zen/layout buttons clicked in a real browser (screenshots set the state through localStorage), and Safari/Firefox.
- Nothing committed.

## 2026-11-17: hide "On this page" from the right side

**Request:** an option to disable "On this page", on the right side itself.

**Done**
- One small icon button fixed at the top right (`.toc-toggle`, `data-toggle="toc"`) shows and hides the list: accent-coloured while shown, grey while hidden. Same `toc: "hidden"` reader setting as the Aa panel's switch. Not shown in zen mode, on screens too narrow for the list, or on pages without headings. (A first version with a × in the heading plus a "show" tab at the edge was replaced at the user's request.)
- Guide: "The website" lists the button. Test extended in `TestZenModeAndPageTOC`.

**Status:** tests pass; shown and hidden states checked in headless Chrome. Nothing committed.

## 2026-11-18: zen mode fills the window

**Request:** in zen mode, with both side menus gone, the text should use the full width of the screen, enlarged to fit.

**Done**
- `book.css`: in zen mode the 90rem layout cap is removed and the type size follows the window (`--zen-size: clamp(1.0625rem, 86vw / measure, 2.25rem)`), so the column reaches across the screen while keeping the normal number of characters per line (`--measure`: 41em; Width narrow 34, wide 53). Headings, code and the pager scale with it. The reader's text size scales only the type, so the column stays full width and a smaller size fits more per line. Phones keep the normal size.
- Guide ("The website") describes it; `TestZenModeAndPageTOC` checks the new rules.

**Status**
- `go vet`, `go test ./...` pass. Checked in headless Chrome at 700, 1280, 1920 (normal, wide, 80% text) and 2560 px. At 2560 px the 2.25rem size limit is reached, so the column leaves side margins.
- Nothing committed.

## 2026-11-18: chapter list "Expand all" / "Collapse all"

**Request:** a close all / open all toggle for the left chapter list; asked what the default should be.

**Done**
- Default changed: only the groups holding the current page are open (previously every top-level part was open too). A long book now shows its parts plus where the reader is.
- A small "Expand all" / "Collapse all" pill at the top of the chapter list, only in books with parts (`Groups` on the page data). The label follows the actual state, including parts opened or closed by hand. Expand all is remembered (`saved.nav = "open"` in `octavo-reader`), so following pages open fully too; Collapse all closes every part and clears it. Aa "Reset" clears it as well.
- Guide: "Chapters and parts" and the website controls table. Test: `TestSidebarParts`.

**Status**
- `go vet`, `go test ./...` pass. Both states checked in headless Chrome on the guide.
- Nothing committed.
- Follow-up: the text pill became a small icon toggle (same style as the "On this page" button): chevrons apart = expand all, chevrons together = collapse all (Material `unfold_more`/`unfold_less`). Label and tooltip "Expand all parts" / "Collapse all parts". Guide wording updated. Tests pass; both icons checked in headless Chrome.

## 2026-11-18: Typst PDF engine (experiment)

**Request:** build the PDF with Typst and see how it fares on all of mynotes; if Typst is missing, fall back to Chrome.

**Done**
- `format: pdf: engine: auto | typst | chrome` (default auto: Typst if installed, else Chrome; a book with its own PDF `css:` keeps Chrome). `$OCTAVO_PDF_ENGINE` overrides; `typst:` / `$OCTAVO_TYPST` name the program. `engine: typst` with no Typst is an error, never a silent fallback. `command:` still overrides everything.
- `internal/format/pdf/typst.go`: chapters are rendered to HTML as before, then converted to Typst markup (headings with labels, lists, task boxes, tables with spans/alignment, code with language, inline code, links checked against every label, real footnotes at the foot of the page, figures, images incl. inline SVG, callouts, quotes, kbd, etc.). `book.typ`: the layout (title page, contents with page numbers and dot leaders, part pages, chapter openings without running header, running header, roman/arabic page numbers, footnotes reset per chapter).
- Code blocks whose lines are too wide are set smaller (down to 70%); longer unbreakable runs and long inline code get zero-width break points. Inline code uses `highlight`, not `box` (a box cannot break).
- `markdown`: highlighted code blocks now carry `data-lang` on `<pre>` (for Typst's own highlighting).
- `fonts.WriteTTF`: the bundled WOFF2 fonts converted to TTF for Typst (new dependency `github.com/tdewolff/font`). Only the Latin subset: the Latin Extended file has the same family name.
- `OCTAVO_TYPST_KEEP=1` keeps the generated Typst source for debugging.
- Tests: `TestPDF` runs both engines where installed (Typst also builds the whole guide); `TestPDFTypstMissing`.

**Results on mynotes (14 books, ~13,900 pages, in a scratch copy)**
- Typst: 41 s for all 14 books, 202 MB. Chrome: 411 s, 463 MB.
- Found a Chrome-engine bug: one too-wide element makes Chrome shrink the whole book (Maths body text printed at 2/3 size; NixOS, Python, Networking, Rust also scaled). Typst sets the size asked for.
- Text into the right margin: 5 pages left of ~13,900 (a chapter title, a few table cells).

**Not done / open**
- Guide pages not yet updated for `engine:` (the "What a browser cannot do" section is now out of date for Typst).
- `highlight:` code colour style is ignored by Typst (Typst's own colours). LaTeX math (`$...$`) is shown as text in both engines.
- No "which engine made this PDF" line yet: logging goes to the warnings stream, so it was taken out.

**Status:** `go vet`, `go test ./...` pass. Nothing committed.

## 2026-11-18 — Code font, sidebar numbers, distinct card colours

- Code font: bundled JetBrains Mono v2.304 (OFL), subset to Latin plus arrows, box drawing and shapes, one variable WOFF2 file per style. It is used on the website (`--font-mono`), in Chrome PDFs and in Typst PDFs (converted to TTF), with ligatures turned off. EPUB is unchanged. Each website gains 3 files (incremental test counts updated). Guide: 03-formats/02-website.md.
- Sidebar numbers: new html setting `sidebar-numbers` (default false). The chapter list now shows titles only; chapter headings keep their numbers (`number-chapters`). Test: TestSidebarNumbers; guide reference table updated.
- Portal card colours: `hues` was replaced by `colours`. Each card has an OKLCH hue and a tone (pale or deep), and each book gets the colour farthest, in OKLab, from those already taken. Hues stay at least 20° apart while room allows, which holds for at least 14 books. Adding a book never changes the colours of the earlier ones. On mynotes the closest pair went from 0.023 to 0.058 (OKLab distance).

## 2026-11-18 — Redirects, GitHub Actions, versions, offline reading

- Moved chapters: the html format keeps `octavo-pages.txt` (page, tab, title, or `-> target`) beside octavo.yml, written by real builds only, not the preview. A page that is no longer built forwards to the chapter with the same title, else the same file name, else to `format: html: redirects:`, which also takes `#section` and full URLs. Chains are followed. A page matching nothing is warned about once, then forgotten. Forwarding pages carry `<meta name="octavo-redirect">`, and the library sitemap leaves them out (`html.IsRedirect`). Code: internal/format/html/redirect.go.
- GitHub Actions: `octavo publish --setup github` writes `.github/workflows/octavo.yml` at the git top level and never overwrites it. The workflow: checkout v7 with full history, setup-go v7, `go install …/octavo@latest`, setup-typst v5, configure-pages v6, `octavo render`, upload-pages-artifact v5, deploy-pages v5. The action versions were checked with `gh api` on 2026-11-18. New env var `OCTAVO_SITE_URL` overrides `site-url:` (book and library). Code: internal/scaffold/github.go.
- Versions: `versions:` in octavo.yml, each entry with label, ref and folder; the first is the latest. internal/versions builds each version into its own folder, from the working copy or from a `git worktree` of the ref that is removed afterwards. Every page of the latest version also gets a forward at its address without a version. The 404 page and sitemap go at the root. Pages get a `<select id="versions">` (same page in the other version via a HEAD check, else that version's index), and older versions show a note. In a library, the latest version's PDF and EPUB move to public/pdf and public/epub; older versions keep their own. The preview ignores versions.
- Offline reading: `sw.js` at each book's root, unless `offline: false` or in the preview. HTML and JSON are network-first; `assets/` is cache-first in a cache named by build hash; other files are stale-while-revalidate. "Save for offline reading" in the sidebar caches every page in search.json plus the images on those pages, 4 at a time. The page unregisters a worker when offline is off. Tested in headless Chrome over CDP: with the server stopped, an unvisited saved chapter loads with its CSS, and an unsaved page shows the offline notice.
- Guide: website page (Reading offline, Moved chapters), new 05-publishing/02-versions.md, gh-pages page (GitHub Actions), and reference tables (offline, redirects, versions, engine, `--setup`, OCTAVO_SITE_URL, OCTAVO_PDF_ENGINE). Updated docs/todo.md.
- Tests: internal/cli/website_test.go (redirects, setup github, site URL from env, versions, offline). Incremental counts are now 30/60 (sw.js).

**Status:** `go vet` and `go test ./...` pass. Nothing committed.

## 2026-11-18 — New chapters join by their number (auto-add)

- Asked for: Octavo adds files that octavo.yml does not list, by their number, as the books repo's update-index.sh did for Quarto. Files without a number are reported, not added.
- internal/book/autoadd.go runs in book.LoadCached, after the listed chapters are parsed, so render, preview and check all see the result.
  - Unlisted `.md` files in the folders of listed chapters go after the sibling with the next lower number (index.md first, then by number, then by name).
  - New numbered folders beside chapter folders become a group, placed among the sibling groups by number. A group's folder is the single subfolder holding all its chapters. Its title comes from index.md, else the folder name in title case. Subfolders become nested sections.
  - Left alone: names starting with `_` or `.`, README.md, included files, the output folder, and nested books. Unnumbered files go to `Book.NotAdded`.
  - octavo.yml is never written. A cache is always used, so the second walk does not parse anything again.
- Reporting:
  - RenderAll warns about each NotAdded file.
  - `render` and the library log say how many chapters were added.
  - `check` gives the reason for left-out files and now skips `_`-prefixed files and folders (Python's 73 `_domains` warnings are gone).
- `book: auto-add: false` turns it off.
- Behaviour change: taking a numbered chapter out of the list no longer removes it; rename it with `_` in front. TestIncrementalBook was updated for this. New tests: TestAutoAddChapters, TestAutoAddFolders.
- On mynotes: Linux went from 425 to 438 chapters (the 13 pending pages, each in its right place); no other book changed.
- Guide: 02-writing/01-chapters.md "New chapters join by their number"; `auto-add` in the reference.

## 2026-11-18 — README and docs brought up to date

- README: a "What you get" list (website with offline reading, EPUB, PDF by Typst or Chrome, libraries, publishing incl. GitHub Actions, auto-add, redirects, versions); install says PDFs need Typst or a browser; `octavo check` and `--setup github` in Start.
- docs/product.md: status line; PDF engine decision rewritten for Typst + Chrome; new decisions (chapter list / auto-add, stable addresses); features for check, auto-add, offline, redirects, versions, portal colours, JetBrains Mono, GitHub Actions; Next now points at todo.md; the module-path open question is gone (it is github.com/kanthi/octavo).
- docs/todo.md: auto-add ticked; what is still undocumented in the guide listed; the include-in-inline-code bug added.
- Guide: install and PDF pages describe both engines (comparison table, `engine`, `typst` settings, finding Typst); troubleshooting has the new PDF error, "not added" and "a new chapter does not appear"; a reference section for `octavo check`; front page summary updated; `OCTAVO_TYPST` in the environment page; `typst` in book settings.
- `octavo render -h` mentions Typst and auto-add.
- guide_test now requires `check` in the commands reference.

## 2026-11-18 — Licence, version 0.1.0, docs on GitHub Pages

- MIT `LICENSE` (Copyright (c) 2026 Kanthi). README "Licence" section: the bundled fonts are under the SIL OFL (licence files in internal/format/fonts/files), and the FreeType credit line that golang/freetype's FreeType License asks for.
- Version 0.007 → 0.1.0 (`cli.Version`), tagged `v0.1.0`, so `go install …@v0.1.0` works. Later versions follow semver: features bump the minor, fixes the patch.
- `.github/workflows/docs.yml`: on every push to main, run the tests, build Octavo from that commit, render the guide (website, PDF with Typst, EPUB) and deploy it to GitHub Pages at https://www.kanthi.in/octavo/ (the account's own domain; kanthi.github.io/octavo forwards there). Pages' source set to "GitHub Actions".
- `guide/octavo-pages.txt` committed, so redirects for moved guide chapters survive CI's clean checkouts.
- First CI run: TestFormats and TestPDF/chrome failed. GitHub's Ubuntu runner has Chromium, which aborted because Ubuntu's AppArmor stops it from setting up its sandbox. The PDF format now retries once with `--no-sandbox` on Linux when the browser fails that way, and errors show the browser's FATAL/ERROR lines instead of the tail of the stack trace.
- The guide went live at https://www.kanthi.in/octavo/. Two fixes after the first deploy:
  - The front page showed `{{octavo-version}}`: only `octavo guide` replaced it. Now `{{< meta octavo-version >}}` is a value every book can use, filled in by the Octavo building it (`book.OctavoVersion`, set in `cli.Run`); guide.WriteTo no longer rewrites text.
  - The sitemap and canonical/og:url used http://, because configure-pages reports http when the repository does not enforce HTTPS. docs.yml now turns it into https://.
- `--setup github` workflow: the same http→https fix as docs.yml, so books published this way get https:// sitemap and link-preview addresses.

## 2026-11-18 — Portal chapter counts; mynotes rebuilds only changed books

- Bug: portal cards counted only the chapters listed in octavo.yml, so Linux showed 425 instead of 444 (19 added by number). Library now keeps `bookFacts` (chapter count and newest date) from the loaded book, built or not; the card falls back to the list only when the book does not load. Test: TestPortalCountsAddedChapters (full build, then a build of another book only).
- mynotes `.github/workflows/octavo.yml` (in that repo): the site from the last build is kept with actions/cache (public/ plus `.octavo-build`, "<commit> <octavo version>"); a push rebuilds only the books with changed files (`octavo render A B`), and everything when there is no cache, a file outside books/ changed, a book was removed or renamed, Octavo's version changed, or "Run workflow" asks for it. A failed partial build falls back to a full one. The decision step was tested on 9 cases in a scratch clone; `git diff --no-renames` was needed so a renamed book's old folder is seen.

## 2026-11-18 — Release 0.1.1

- Version 0.1.1, tagged `v0.1.1`. Fixes since 0.1.0: Chrome PDFs on Ubuntu 24.04+ (no sandbox when AppArmor forbids it), `{{< meta octavo-version >}}` for every book, https:// addresses in the `--setup github` workflow, and portal cards counting chapters added by number.
- mynotes' workflow installs `octavo@latest`, sees the new version, and rebuilds every book once.
