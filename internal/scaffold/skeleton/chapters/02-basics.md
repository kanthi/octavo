---
title: The basics
---

A chapter's title comes from the `title` in its front matter, as here, or from its first `# Heading`.

## Code

Name the language after the opening fence to get colours. Every block has a copy button.

```go
package main

import "fmt"

func main() {
	fmt.Println("Hello, book")
}
```

## Tables

| Command | What it does |
|---|---|
| `octavo preview` | Shows the book locally |
| `octavo render` | Builds the book |
| `octavo publish` | Puts the book online |

## Task lists

- [x] Create the book
- [ ] Write the first chapter
- [ ] Publish it

## Your own look

Open `custom.css` and change a colour or a font. It loads after the built-in theme, so anything you set there wins.
