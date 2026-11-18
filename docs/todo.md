# To-do

Gaps found on 2026-11-17 by comparing Octavo with Quarto, Pandoc, mdBook and GitBook. Ticked items are done; the rest are waiting for a decision or their turn. Updated 2026-11-18.

## Now (2026-11-17)

- [x] `octavo check`: broken links and `#` targets, missing images, chapter files not in the chapter list, unused images, images without alt text
- Quick wins
  - [x] Smart typography: curly quotes, dashes, ellipsis
  - [x] Heading links: a link icon beside each heading to copy its address
  - [x] ← and → keys for the previous and next chapter
  - [x] Favicon (`favicon:` setting, with a default)
  - [x] `sitemap.xml` (needs the site's address: `site-url:`)
  - [x] A 404 page
  - [x] Social preview cards (Open Graph and Twitter meta tags)
- Writing essentials
  - [x] Figures: an image alone in a paragraph becomes a numbered figure with a caption
  - [x] Image size: `{width=60%}` after an image
  - [x] Light and dark images: `name-dark.svg` beside `name.svg` is used on dark pages
  - [x] Includes: `{{< include file.md >}}`
  - [x] Variables: `{{< var name >}}`, set under `variables:` in `octavo.yml`
  - [x] Content for some formats only: `::: {.content-visible when-format="pdf"}`
  - [x] Dates: when the book was created and last updated, and "Last updated" on each chapter
- [ ] Document all of the above in `octavo guide`: later, once the feature set is final (decided 2026-11-17). Done so far: `octavo check` (reference), dates. Still missing: smart typography, heading links, figures, image size, light and dark images, includes, variables, format-only content, favicon, sitemap, 404, social cards.

## Later

### Getting Octavo to people
- [ ] Ready-made downloads for macOS, Linux and Windows (GoReleaser), and `brew install`
- [x] A `LICENSE` file: MIT, 2026-11-18
- [ ] Test on Windows: paths, preview, finding a browser
- [ ] Decide what of `AGENT_WORKLOG.md` belongs in a public repository

### Writing
- [ ] Maths: TeX (`$…$`, `$$…$$`) to MathML at build time, with a bundled math font
- [ ] Diagrams to SVG at build time: D2 built in, Mermaid through the headless browser, Graphviz when `dot` is installed; cached by content
- [ ] Cross-references: "Figure 3", "Table 2", "Section 4.1", numbered automatically
- [ ] Citations and a bibliography from BibTeX
- [ ] Code: line numbers, highlighted lines, a file-name title, code included from a source file
- [ ] Callouts: custom titles, callouts that fold
- [ ] Appendices numbered A, B, C; dedication and copyright pages
- [ ] Back-of-book index
- [ ] Animated SVG (asked 2026-11-18). Already works: inline `<svg>` with `<animate>` or CSS `@keyframes`, or `![](file.svg)`. It plays on the website and in some EPUB readers; the PDF shows the first frame. To do:
  - a guide section, "Animations and SVG": how to add one, what each format shows, and to draw a sensible first frame for the PDF
  - a `prefers-reduced-motion` rule in the built-in theme that stops animations for readers who turn motion off
  - tips: use `currentColor` so the SVG follows light and dark, and keep inline SVG valid XML for the EPUB
- [ ] Right-to-left languages
- [x] New chapters join by their number (`auto-add`), unnumbered ones reported; `octavo check` skips `_` names, 2026-11-18
- [ ] Chapter-level drafts
- [ ] Includes are expanded even inside inline code, so a chapter cannot show an include shortcode as an example. Needs an escape, or skipping code spans
- [ ] Heading ids the way Pandoc and Quarto make them (keep dots: `go-1.27`, not `go-127`), so links in imported books keep working. Found by `octavo check` on the imported Go book: 5 broken links.

### Website
- [x] Old addresses of moved chapters forward to the new place (`octavo-pages.txt`, `redirects:`), 2026-11-18
- [x] Versions side by side, from git refs, with a version menu (`versions:`), 2026-11-18
- [x] Offline reading: a service worker, and "Save for offline reading", 2026-11-18
- [ ] The whole book on one page (like mdBook's print page): Ctrl+F across the book, quick printing
- [ ] `llms.txt`, and the whole book as one Markdown file, for AI tools
- [ ] Tabs in chapters (macOS / Linux / Windows); the PDF and EPUB show every tab in turn
- [ ] Pop-up definitions for terms, and a glossary page made from the same list
- [ ] "Was this page helpful?", sent to an analytics tool the author chooses
- [ ] Share images drawn for each chapter (book title and chapter), for `og:image`
- [ ] Translations: one book in several languages, with a language menu
- [ ] "Edit this page" link
- [ ] Remember where the reader stopped; reading time
- [ ] Search: highlight matches on the page; test speed on a 1000-chapter book
- [ ] `head-html:` for analytics or comments (giscus)
- [ ] Portal order: a `books:` list in `octavo-library.yml`; `--skip` for one run

### Output and speed
- [x] Build a library's books at the same time (`--jobs`): the three test books in every format went from 95 s to 53 s
- [x] Faster PDFs for big books: the Typst engine (`engine: auto|typst|chrome`), done 2026-11-17. All of mynotes: 41 s with Typst, 411 s with Chrome. Maybe bundle Typst later through WebAssembly (wazero)
- [ ] Smaller PDFs (about 28 KB per page now)
- [x] Page numbers on the PDF contents page (Typst engine)
- [ ] Word (`.docx`) output
- [ ] Incremental `octavo render` (only the preview is incremental now)
- [ ] Validate the EPUB with `epubcheck`; open it in Apple Books

### Workflow
- [ ] `octavo import` for Quarto books (the importer in `~/Workspace/RnD/octavo-test/tools/` is a start)
- [x] A GitHub Actions workflow file that publishes on every push: `octavo publish --setup github` (2026-11-18)
- [ ] Starter templates: `octavo new --template novel|technical|course`
- [ ] Build hooks: run a script before or after a build
- [ ] Test `ftp` against a real server, and `gh-pages` and the GitHub Actions workflow against GitHub (the workflow needs Octavo's repository to be public for `go install`)
- [ ] Preview addresses for pull requests (Netlify, Cloudflare Pages): each branch on its own address
- [ ] Free samples: publish some chapters as a sample beside the full book (as Leanpub does)
