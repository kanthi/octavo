# Drafts

A book that is not ready can be marked as a draft in its own `octavo.yml`:

```yaml
book:
  title: Rust
  draft: true
```

## What a draft means

A draft is left out of every build unless you ask for drafts with `--drafts`:

| Command | Without `--drafts` | With `--drafts` |
|---|---|---|
| `octavo render` in a library | The draft is skipped, and the run lists it | Every book is built |
| `octavo render Rust` | Refused: "Rust is a draft" | Built |
| `octavo preview` | Left out | Shown, with a **Draft** badge on the portal |
| `octavo publish` | Left out | Published |
| A single book marked as a draft | Refused | Built |

Naming a draft on the command line is not enough; `--drafts` is always needed. That way a draft cannot be published by mistake.

## Nothing left behind

A build without `--drafts` also deletes any copy of a draft from a library's `public/` folder, left there by an earlier build with `--drafts`. So `octavo publish --no-render`, which sends that folder as it is, cannot send a draft either.

> [!NOTE]
> Drafts are whole books. Single chapters cannot be marked as drafts; leave a chapter out of the `chapters:` list until it is ready.
