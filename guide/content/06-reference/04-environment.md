# Environment variables

Octavo does not read `.env` files. Set these in your shell, or in your CI system's settings.

| Variable | Meaning |
|---|---|
| `OCTAVO_TO` | The formats to build when `--to` is not given, such as `html` or `pdf,epub` |
| `OCTAVO_BROWSER` | The browser program that prints PDFs, when Octavo should not look for one |
| `OCTAVO_FTP_PASSWORD` | The password for the `ftp` publish target. `password-env` in its settings names another variable. |
| `OCTAVO_SITE_URL` | The website's address, overriding `site-url:`. The GitHub Actions workflow sets it. |
| `OCTAVO_TYPST` | The Typst program that sets PDFs, when Octavo should not look for one |
| `OCTAVO_PDF_ENGINE` | The PDF engine, overriding `engine:`: `typst`, `chrome` or `auto` |
| `OCTAVO_SITE_DIR` | Set **by** Octavo for a `command` publish target: the folder of the built book |

The `gh-pages` and `rsync` targets use your existing git and SSH credentials and need no variables.
