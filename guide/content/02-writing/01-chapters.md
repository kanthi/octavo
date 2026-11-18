# Chapters and parts

A chapter is one Markdown file. The `chapters:` list in `octavo.yml` decides which files are in the book and in what order, and [new numbered files join by themselves](#new-chapters-join-by-their-number).

## The chapter list

An entry in the list can be written three ways:

```yaml
book:
  chapters:
    - index.md                          # 1. a chapter: just its file
    - text: Getting going               # 2. a chapter with its own label in the sidebar
      file: chapters/start.md
    - part: Commands                    # 3. a group of chapters
      contents:
        - chapters/ls.md
        - chapters/cd.md
```

`part:` and `section:` mean the same thing. Use whichever word reads better. Groups can hold other groups to any depth, which is how this guide's [Publishing](../05-publishing/01-publishing.md) part has its own "Targets" section:

```yaml
    - part: Publishing
      contents:
        - content/05-publishing/01-publishing.md
        - section: Targets
          contents:
            - text: GitHub Pages
              file: content/05-publishing/targets/gh-pages.md
```

On the website, a group is a heading in the sidebar that opens and closes. Only the groups around the page you are reading are open, so a long book shows its parts and where you are. The small chevron button at the top of the list opens every group (expand all), and they stay open from page to page until you press it again (collapse all). In the PDF, each part gets a page of its own; in the EPUB, each part and section gets one too.

Chapter files can sit in any folders you like, as long as they are inside the book's folder. Folders do not have to match the parts.

## New chapters join by their number

You do not have to list every new chapter. A Markdown file next to the book's chapters joins the book by itself when its name starts with a number, such as `06-sensors.md` or `01a-why.md`. It goes after the chapter of the same folder with the next lower number, so a folder of numbered files stays in order:

```text
content/10-hardware/
├── 05-hdparm.md        listed in octavo.yml
├── 06-sensors.md       new: added after 05-hdparm.md
└── notes.md            new, no number: left out, with a warning
```

A new numbered **folder** beside the book's chapter folders becomes a new part (or section, inside a part), placed among the others by its number. Its title is the title of its `index.md`, or else its name: `17-modern-tools` becomes "Modern Tools". Its numbered files are its chapters, and its numbered subfolders its sections.

`octavo.yml` is not changed: the files join each time the book is built, in `render`, `preview` and `check`. `octavo render` says which chapters it added. Listing a file in `octavo.yml` is still the way to give it a place of its own, or a sidebar label.

Some files are always left alone:

- names starting with `_` or `.`, such as `_notes.md` or a `_planning/` folder: set aside on purpose, and `octavo check` does not mention them;
- `README.md` files;
- files that a chapter pulls into itself with an `include`.

A file without a number is not added, because there is no telling where it belongs. `render`, `preview` and `check` say so: give it a number, list it in `octavo.yml`, or put `_` in front of its name to keep it out quietly. To take a numbered chapter out of the book, do the same: removing it from the list is not enough, because its number brings it back.

To turn all of this off, and use only the list:

```yaml
book:
  auto-add: false
```

## A chapter's title

The title comes from the chapter's first line when that line is a level-1 heading:

```markdown
# Getting going

The text of the chapter...
```

The heading is taken out of the text and used as the title: at the top of the page, in the sidebar, in the browser tab and in the contents of the EPUB and PDF. A title set in front matter wins over the heading (see below).

## Numbers

Chapters are numbered 1, 2, 3 in reading order, across parts. Two kinds of chapter have no number:

- **The preface:** an `index.md` at the start of the list. It is the book's first page.
- **Any chapter with `unnumbered: true`** in its front matter, such as an appendix or a glossary. This guide's [Troubleshooting](../troubleshooting.md) page is one.

To number nothing at all, set `number-chapters: false` under the format (see [Book settings](../06-reference/02-book-settings.md)).

## Front matter

A chapter can start with a block of settings between two `---` lines:

```markdown
---
title: Questions and answers
unnumbered: true
---

The text starts here.
```

| Key | Meaning |
|---|---|
| `title` | The chapter's title, in place of its first heading |
| `unnumbered` | `true` to give the chapter no number |

## "On this page"

Level-2 and level-3 headings (`##` and `###`) in a chapter are listed beside it on the website, under "On this page", and each gets its own link address. Readers can hide this list (see [The website](../03-formats/02-website.md)), and you can turn it off for the whole book with `page-toc: false`.
