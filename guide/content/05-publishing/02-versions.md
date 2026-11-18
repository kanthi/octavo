# Versions

A book can stay online in several versions at once, such as a 2nd and a 1st edition, or a course as taught in 2026 and in 2025. Each version gets its own folder, and every page gets a menu beside the title to switch between them. Readers of an older version see a note leading to the latest.

```yaml
versions:
  - label: 2nd edition          # the book as it is now
  - label: 1st edition
    ref: v1                     # the book as it was at the git tag v1
```

The first version listed is the **latest**. `octavo render` then builds:

```text
_book/
├── 2nd-edition/      the book as it is now
├── 1st-edition/      the book at tag v1
└── index.html, chapters/...   forward to 2nd-edition/
```

Addresses without a version forward to the latest, so links made before the book had versions keep working, and always lead to the newest text.

## Where versions come from

- A version **without** `ref` is the book as it is in its folder now. Only one version can be like that.
- A version **with** `ref` is built from that git branch, tag or commit. Octavo checks it out into a temporary folder (with `git worktree`), so your working folder is never touched. Each version is built with its own settings and chapters, as they were then.

A typical way to work: when an edition is done, tag it (`git tag v1`), and add it to the list.

| Key | Meaning | Default |
|---|---|---|
| `label` | The name in the menu. **Required.** | |
| `ref` | A git branch, tag or commit | The book as it is now |
| `folder` | Its folder in the output | Made from the label: `2nd edition` → `2nd-edition` |

## Good to know

- Switching versions in the menu opens the same chapter in the other version, when it has it, and that version's first page otherwise.
- Every version is built in every format, so each has its own PDF and EPUB. In a library, the latest version's PDF and EPUB are the ones on the portal.
- The preview shows the book as it is now, without versions.
- Building on GitHub Actions needs the git history, which the workflow from `octavo publish --setup github` fetches.
