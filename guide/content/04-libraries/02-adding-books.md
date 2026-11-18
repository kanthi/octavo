# Adding books

`octavo new` is only for starting out. Once a library exists, books are added with `octavo add`.

> [!IMPORTANT]
> `octavo add` never overwrites or changes a book that is already there. If any name you give is taken, by a book or by any other file or folder, in any letter case, the whole command stops before it creates anything.

## A new, empty book

```sh
cd my-library
octavo add Rust
octavo add NixOS Networking      # several at once
```

Each book starts from the same sample as `octavo new`, in `books/<Name>/`. Its `octavo.yml` has no `publish:` section, because the library publishes all its books together.

A book's name is its folder name and also its address on the site (`html/<Name>/`), so it may use letters, digits, `-`, `_` and `.`, starting with a letter or digit.

## A book you already have

```sh
octavo add --from ../my-book                # copied as books/my-book
octavo add --from ../my-book Linux          # copied as books/Linux
```

`--from` **copies** the book into the library. The original is not touched in any way. The copy leaves out the book's built output (`_book/`) and version-control folders such as `.git`.

If the copied book has its own `publish:` settings, Octavo tells you they are not used inside a library. If it is a draft, it tells you that too.

## Turning a single book into a library

There is no command that moves a book, so nothing can be lost. Make a library beside the book, and copy the book in:

```sh
octavo new --library my-library
octavo add --dir my-library --from my-book
```

When you are happy with the library, the original `my-book` folder can be kept or deleted; that choice stays yours. Running `octavo add` inside a single book prints these two commands as a reminder.

## The rules, in short

| You run | In or under a library | In a book's own folder | Anywhere else |
|---|---|---|---|
| `octavo new <dir>` | Refused: use `add` | Refused: it holds a book | Makes a book |
| `octavo new --library <dir>` | Refused | Refused: it holds a book | Makes a library |
| `octavo add <Name>` | Adds a book | Refused, with the steps above | Refused |
