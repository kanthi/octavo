# A folder

Copies the built book into a folder on this computer: a web server's folder, a synced drive, a backup.

```yaml
publish:
  folder:
    path: /var/www/book
```

| Setting | Meaning |
|---|---|
| `path` | The folder; a relative path is relative to the book's folder |

Files are copied over what is there. Nothing else in the folder is deleted.
