# Sample library

Five small books laid out like a multi-book Quarto repository: one folder per book under `books/`, each with its own `octavo.yml`, `index.md`, `content/` and `styles/`.

```sh
octavo render --dir examples/library          # every book, then the portal
octavo render --dir examples/library Linux    # one book; the portal keeps the others
octavo preview --dir examples/library --open  # portal and books with live reload
```

Each book builds into `books/<Name>/_book/` and is copied to `public/html/<Name>/`. `public/index.html` is the portal.

`books/Networking` is the deep-structure test: its chapter list goes four levels down (part, section, sub-section, chapter), the files are nested the same way, and chapters link to each other across levels.
