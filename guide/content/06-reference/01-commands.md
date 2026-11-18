# Commands

```text
octavo <command> [flags] [arguments]
```

Flags can come before or after the arguments. `octavo help` lists the commands; `octavo <command> -h` lists a command's flags.

## octavo new

Start a book or a library. Only for starting out: it refuses a folder inside a library, where books are added with `octavo add`.

```sh
octavo new my-book
octavo new --library my-library
octavo new --library my-library --books Linux,Go
```

| Flag | Meaning |
|---|---|
| `--library` | Make a library instead of a book |
| `--books <A,B>` | With `--library`: books to create in it, separated by commas |

## octavo add

Add books to a library. Never overwrites or changes anything already there. See [Adding books](../04-libraries/02-adding-books.md).

```sh
octavo add Rust NixOS
octavo add --from ../my-book [Name]
```

| Flag | Meaning |
|---|---|
| `--dir <path>` | The library folder (default: the current folder) |
| `--from <path>` | Copy this existing book into the library; the original is left as it is |

## octavo render

Build the book into `_book/`, or every book of a library into `public/`.

```sh
octavo render
octavo render --to html
octavo render Linux Go          # in a library: only these books
```

| Flag | Meaning |
|---|---|
| `--dir <path>` | The book or library folder (default: the current folder) |
| `--to <formats>` | `all`, or a comma-separated list of `html`, `epub`, `pdf` (default: `$OCTAVO_TO`, then `octavo.yml`) |
| `--format <formats>` | Another name for `--to` |
| `--drafts` | Include books marked as drafts |
| `--jobs <number>` | In a library, how many books build at the same time (default: half the processor cores, at most 4). Use `--jobs 1` to build one at a time |

Books of a library build at the same time, so a full build takes about as long as its slowest book. Each book's messages still appear in alphabetical order.

## octavo preview

Serve the book or library locally, rebuild what changes on every save, and reload the browser. `octavo dev` is another name for it. See [Live preview](../01-start/03-preview.md).

| Flag | Meaning |
|---|---|
| `--dir <path>` | The book or library folder (default: the current folder) |
| `--port <number>` | The port (default 4000) |
| `--open` | Open the browser |
| `--drafts` | Include books marked as drafts |

## octavo publish

Build, then send the result to a publish target. See [Publishing](../05-publishing/01-publishing.md).

```sh
octavo publish
octavo publish gh-pages
```

| Flag | Meaning |
|---|---|
| `--dir <path>` | The book or library folder (default: the current folder) |
| `--to <formats>` | The formats to build, as for `render` |
| `--format <formats>` | Another name for `--to` |
| `--drafts` | Include books marked as drafts |
| `--jobs <number>` | In a library, how many books build at the same time, as for `render` |
| `--no-render` | Send what is already built, without building again |
| `--setup github` | Instead of publishing: write a GitHub Actions workflow that publishes on every push. See [GitHub Pages](../05-publishing/targets/gh-pages.md). |

## octavo check

Look for problems without building anything. Errors make the command fail, so it can guard a CI job.

```sh
octavo check                # the book, or every book of a library
octavo check Linux Go       # in a library: only these books
```

It reports:

- **Errors:** links to files or chapters that do not exist, links to a `#place` that is not in that chapter, missing images.
- **Warnings:** links that point outside the book folder, images without alt text, images nothing uses, and chapter files that are not in the book, with the reason (for example, a new file whose name has no number).

Files and folders whose names start with `_` are set aside on purpose, and not reported.

| Flag | Meaning |
|---|---|
| `--dir <path>` | The book or library folder (default: the current folder) |
| `--quiet` | Report errors only, not warnings |

## octavo guide

Open this guide. It is built into the `octavo` program, so it always matches the version you run.

```sh
octavo guide
octavo guide --to pdf
octavo guide --copy my-guide
```

| Flag | Meaning |
|---|---|
| `--to <formats>` | Save the guide as `pdf`, `epub`, or `pdf,epub` instead of opening it |
| `--out <path>` | With `--to`: the folder to save in (default: the current folder) |
| `--copy <path>` | Write the guide's source into this new folder, as a book to edit and render |
| `--port <number>` | The port (default: any free port) |
| `--no-open` | Serve the guide without opening the browser |

## octavo version

Print the version.
