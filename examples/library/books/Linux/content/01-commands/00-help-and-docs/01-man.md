# man

`man` shows the manual page for a command.

```bash
man ls          # the manual for ls
man 5 passwd    # section 5: the file format, not the command
man -k copy     # search page descriptions (same as apropos)
```

## Sections

| Section | Holds |
|---|---|
| 1 | User commands |
| 5 | File formats |
| 8 | Administration commands |

> [!TIP]
> Inside a page, press `/` to search and `q` to quit.

When you do not know the command's name, use [apropos](02-apropos.md).
