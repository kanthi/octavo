# Publishing

A built book is plain files with relative links, so any web host can serve it. `octavo publish` builds the book and sends it to a **target** you describe in `octavo.yml`, or in `octavo-library.yml` for a library.

```yaml
publish:
  gh-pages: {}
  backup:
    type: folder
    path: /Volumes/Backup/my-book
```

```sh
octavo publish              # the only target, when there is one
octavo publish gh-pages     # or name it
octavo publish backup
```

## Kinds of target

| Type | Sends the book to |
|---|---|
| [`gh-pages`](targets/gh-pages.md) | A git branch served by GitHub Pages |
| [`ftp`](targets/ftp.md) | A web host, over FTP or FTPS |
| [`rsync`](targets/rsync.md) | Your own server, over SSH |
| [`folder`](targets/folder.md) | A folder on this computer |
| [`command`](targets/command.md) | Anything with its own upload tool: Cloudflare Pages, Netlify, Amazon S3 |

A target's name is its type unless it sets `type:`. So one book can have several targets of the same kind, such as a `staging` and a `live` folder.

## What publish builds

`octavo publish` first does a full build, with the same `--to` and `--drafts` choices as `render`, then sends the result. To send what is already built, without building again:

```sh
octavo publish --no-render
```

For a book this sends `_book/`; for a library, `public/`.
