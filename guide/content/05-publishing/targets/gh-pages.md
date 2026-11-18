# GitHub Pages

There are two ways to publish on GitHub Pages:

- **From GitHub, on every push** (recommended): GitHub builds the book itself whenever you push. Nothing to run on your computer.
- **From your computer**, with `octavo publish`: you build, and Octavo pushes the result to a branch.

## On every push, with GitHub Actions

```sh
octavo publish --setup github
```

This writes `.github/workflows/octavo.yml` at the top of your repository, and prints what is left to do:

1. Once, on GitHub: **Settings → Pages → Build and deployment → Source: GitHub Actions**.
2. Commit the workflow and push.

From then on, every push to your branch builds the book, or the whole library, and publishes it; the **Actions** tab shows each run. GitHub's machine installs Octavo and Typst for the PDF, and sets the site's address, so the sitemap and link previews are right without `site-url:`.

The workflow is an ordinary file: edit it freely. Octavo never overwrites it; delete it to write a fresh one.

> [!NOTE]
> The workflow installs Octavo with `go install github.com/kanthi/octavo/cmd/octavo@latest`, from Octavo's public repository.

## From your computer, to a branch

```yaml
publish:
  gh-pages: {}
```

Octavo puts the built book on a branch of your repository, `gh-pages` by default, as one fresh commit, and pushes it. Nothing on your working branch changes. It also adds a `.nojekyll` file, so that GitHub serves the files as they are.

Then, once, in the repository's settings on GitHub: **Pages → Build and deployment → Deploy from a branch**, and choose `gh-pages`.

| Setting | Meaning | Default |
|---|---|---|
| `remote` | A git remote's name, or a repository URL | `origin` |
| `branch` | The branch to publish to | `gh-pages` |
| `cname` | A custom domain, such as `books.example.com` | |

> [!WARNING]
> Each publish replaces the branch with a single new commit. Do not keep anything else on that branch.

Git uses your usual credentials; Octavo asks for none.
