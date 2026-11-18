# Octavo

A Go command-line tool that turns a folder of Markdown chapters into a book and publishes it. It follows the book workflow of Quarto, but keeps only the parts needed to write and ship a book.

Status (2026-11-18, version 0.1.1, MIT License): HTML, EPUB and PDF output, libraries, preview, checking and publishing are built. In daily use for a library of 14 books (about 2,850 chapters).

## The job

Write chapters in plain Markdown, run one command to get a clean, navigable book, and run one more to put it online. One static binary, no Python, R, Node or LaTeX required. The PDF needs Typst or a Chrome-family browser.

## A book project

```text
my-book/
├── octavo.yml        # book settings and chapter order
├── index.md          # preface / landing page
├── chapters/
│   ├── 01-intro.md
│   └── 02-basics.md
├── images/
├── custom.css        # your own styles (optional)
├── octavo-pages.txt  # every page address the website has had (kept by Octavo; commit it)
└── _book/            # build output (git-ignored)
```

`octavo.yml`:

```yaml
book:
  title: My Book
  author: Jane Doe
  date: 2026-11-14
  chapters:
    - index.md
    - chapters/01-intro.md
    - chapters/02-basics.md
format:
  html:
    theme: auto         # light | dark | auto
    css: custom.css     # one file or a list, loaded after the built-in theme
publish:
  gh-pages: {}
  ftp:
    host: ftp.example.com
    user: me
    path: public_html/book
  cloudflare:           # any name; "type" picks the module
    type: command
    run: npx wrangler pages deploy {dir} --project-name my-book
```

## Commands

| Command | What it does |
|---|---|
| `octavo new <dir>` | Scaffold a book with a sample `octavo.yml` and two chapters. Only for starting out: refused inside a library. |
| `octavo new --library <dir> [--books A,B]` | Create a library, optionally with books in it. |
| `octavo add <Name>...` | Add new books to a library. Never overwrites a book or folder that exists. |
| `octavo add --from <book> [<Name>]` | Copy an existing book into a library; the original is left as it is. This is how a single book becomes a library. |
| `octavo render` | Build every format listed in `octavo.yml` (all three for a new book) into `_book/`. `--to all`, `--to pdf`, `--to html,epub` choose the formats for one run. |
| `octavo preview` | Serve the book locally, rebuild on file change, reload the browser. |
| `octavo publish [target]` | Render, then send `_book/` to a target from `octavo.yml`. The name is optional when there is only one. |
| `octavo publish --setup github` | Write a GitHub Actions workflow that builds and publishes on GitHub Pages on every push. |
| `octavo check` | Broken links and `#` targets, missing images, images without alt text, unused images, and chapter files left out of the book. Builds nothing. |
| `octavo guide` | Open the Octavo Guide, the user guide built into the binary. `--to pdf,epub` saves it; `--copy <dir>` writes its source as a sample book. |

## A library of books

For a repository that holds many books, a library folder builds them all into one site:

```text
my-library/
├── octavo-library.yml    # portal title, books folder, public folder, publish targets
├── books/
│   ├── Linux/            # a normal book; builds into books/Linux/_book/
│   └── Go/
└── public/               # the site to host
    ├── index.html        # the portal: one card per book
    └── html/Linux/, html/Go/
```

`octavo render`, `preview` and `publish` detect the library and work on every book. `octavo render Linux` rebuilds one book and refreshes its copy; the portal keeps the rest. A failing book is reported and skipped; the others are still built, and the command exits with an error.

A book with `draft: true` under `book:` is left out of every build, and off the portal, unless `--drafts` is given, even when it is named on the command line. Builds without `--drafts` delete any earlier copy of a draft from `public/`.

## Decisions

- **Formats:** HTML, EPUB and PDF. `--to` on the command line picks formats for one run; `format:` in `octavo.yml` sets the usual ones.
- **PDF engine:** two engines (2026-11-17), chosen by `engine: auto|typst|chrome` or `$OCTAVO_PDF_ENGINE`.
  - **Typst** typesets the book from Typst markup that Octavo writes from each chapter's HTML. It gives page numbers in the contents, footnotes at the foot of the page, and clean chapter openings, and it is about 10 times faster: all 14 books of the main library take 41 s, against 411 s with Chrome.
  - **A headless Chrome-family browser** prints a single HTML page. It is the fallback, and the choice for books with their own PDF stylesheets (`css:`), which only the browser reads.
  - `auto` prefers Typst when it is installed. `command:` runs any other HTML-to-PDF tool. Typst does not use `highlight:`.
- **Chapter list:** `octavo.yml` lists the chapters, but new numbered files and folders join by their number (`auto-add`, 2026-11-18), as the books repository's `update-index.sh` did for Quarto. octavo.yml is never rewritten. Files without a number are reported. Names starting with `_` are set aside on purpose.
- **Stable addresses:** a page's address never just disappears. `octavo-pages.txt` records every address built; a moved chapter's old address forwards to its new place.
- **Themes:** readers of the code should never need to fork the theme. A book lists its own CSS file, which loads after the built-in theme and overrides its CSS variables. A book can also replace the page template.
- **Hosting:** the output is plain static files with relative links only, so it can be copied to any folder on any web server.
- **Publishing:** several targets, in the style of MkDocs deploy plugins. Built: `gh-pages`, `ftp`, `rsync`, `folder`, and `command` (runs any upload tool, which covers Cloudflare Pages, Netlify and S3 for now). `--setup github` instead hands the build to GitHub Actions and GitHub's own Pages deployment, with no branch. Dedicated Cloudflare and other modules can follow.
- **Modular:** each format and each publish target is a separate package behind a small interface (`format.Format`, `publish.Publisher`), listed in one file, `internal/cli/modules.go`. Each module reads its own section of `octavo.yml`.

## Features

Built:

- **Library:** many books, each with its own build folder, copied into one public folder with a portal page. `new` creates a library (optionally with books); `add` adds new books or copies in an existing one, never overwriting anything.
- **Drafts:** `draft: true` keeps a book out of render, preview, publish and the portal unless `--drafts` is given.
- **Parts and sections:** the chapter list can group chapters, nested to any depth, with a sidebar label per chapter. Only the part holding the current page is open; a chevron button expands or collapses them all.
- **New chapters by number:** unlisted numbered `.md` files beside the chapters, and new numbered folders, join the book in number order; unnumbered ones are reported by render, preview and check. `auto-add: false` turns it off.
- **Check:** `octavo check` finds broken links and anchors, missing images and alt text, unused images, and chapter files left out.
- **Markdown:** CommonMark plus GitHub tables, footnotes, task lists and front matter (`title`, `unnumbered`). Smart quotes and dashes. Figures with captions and `{width=60%}`. Includes, variables, and content for some formats only. Raw HTML, including animated SVG.
- **HTML book:** one page per chapter, sidebar chapter list (titles only; numbers with `sidebar-numbers: true`), "on this page" list, previous/next links and ← → keys, chapter numbering, light and dark themes with a switch, mobile layout, heading links, favicon, sitemap, 404 page, and social preview tags.
- **Offline reading:** a service worker keeps every page read, and "Save for offline reading" keeps the whole book with its pictures. Pages still come fresh from the network when there is one. `offline: false` turns it off.
- **Redirects:** moved chapters forward from their old addresses, found by title or file name, or named under `redirects:`.
- **Versions:** `versions:` builds editions side by side, each from the working folder or a git ref (via `git worktree`), with a version menu, a note on older versions, and unversioned addresses that lead to the latest.
- **User guide:** the Octavo Guide, an Octavo book in `guide/`, embedded in the binary and opened with `octavo guide`. Tests fail when a flag, format or publish target is undocumented, or when the guide has a broken link. The README holds only install and quick start.
- **Zen mode and layout:** readers can hide the chapter list (the ☰ button, on any screen width), "On this page", or both with zen mode (button or Z key, Esc to leave), which also fades the top bar and lets the text fill the window, its size growing with the screen. Remembered with the other reading settings. Authors can drop "On this page" with `page-toc: false`.
- **Reading settings:** an "Aa" menu on every page, like an e-book reader: font (Literata, Source Serif 4, Atkinson Hyperlegible Next, or the system's), text size, line spacing, page width. Remembered per browser. The fonts are bundled, so books work offline and load nothing from other sites.
- **Code:** fenced code blocks with syntax highlighting and a copy button, set in the bundled JetBrains Mono (website and PDF) without ligatures.
- **Callouts:** `> [!NOTE]`, `[!TIP]`, `[!WARNING]`, `[!IMPORTANT]`, `[!CAUTION]`.
- **Images:** images used in chapters are copied into `_book/`. Other files (fonts, downloads) are listed under `format: html: resources:`.
- **Links:** a link to another chapter's `.md` file becomes a link to its page.
- **Search:** client-side search over a JSON index built at render time.
- **Preview:** local server that, on each save, rebuilds only the affected output files and reloads only the browser tabs showing them. `octavo dev` is another name for it. It builds the website only, without versions or offline reading.
- **Portal:** one card per book, each in its own colour, chosen to look as different as possible from the others, with Read, PDF and EPUB buttons, chapter counts and update dates. Books build in parallel (`--jobs`).

- **EPUB:** one EPUB 3.3 file that also opens in EPUB 2 apps: cover (own or generated), title page, nested contents, landmarks, NCX, pop-up footnotes, accessibility metadata, and a stylesheet that leaves font, size and colours to the reading app.
- **PDF:** laid out as a book: title page and contents numbered in roman, a page per part, chapters on new pages, running headers, justified and hyphenated text in a bundled book typeface, book-style tables, bookmarks, working links between chapters. With Typst: page numbers in the contents, footnotes at the foot of the page, and long code lines scaled to fit.
- **GitHub Actions:** `octavo publish --setup github` writes a workflow that installs Octavo and Typst, builds, and deploys to Pages; `OCTAVO_SITE_URL` gives the build the site's address.
- **Download links:** the HTML book links to its PDF and EPUB when they are built in the same run.

## Next

The working list is [todo.md](todo.md). The largest open items:

- **Maths:** TeX to MathML at build time. Formulas are shown as TeX for now, in every format.
- **Diagrams:** Mermaid, D2 and Graphviz to SVG at build time.
- **Cross-references, citations, an index.**
- **Releases:** ready-made downloads, `brew install`, a LICENSE file, and the repository made public, which `go install` and the GitHub Actions workflow need.
- **Validate the EPUB with `epubcheck`** (needs Java; not installed on the development machine).

## Out of scope

- Running code in documents. This is the biggest Quarto feature and Octavo deliberately leaves it out.
- Websites, slides, dashboards, and Jupyter notebooks as input.
- Third-party plugins loaded at run time. Modules are compiled in.

## Open questions

- Should FTP publishing delete server files that are no longer in the book?
- Should Typst be bundled into the binary (through WebAssembly), so that PDFs need nothing installed?
