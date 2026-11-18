# apropos

`apropos` searches the one-line descriptions of every manual page.

```bash
apropos partition
apropos -s 1 archive   # only user commands
```

> [!NOTE]
> If `apropos` finds nothing at all, the index may be missing. Rebuild it with `sudo mandb`.
