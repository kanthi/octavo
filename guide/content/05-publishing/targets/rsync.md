# rsync

For your own server, reached over SSH.

```yaml
publish:
  rsync:
    target: me@example.com:/var/www/book
```

| Setting | Meaning | Default |
|---|---|---|
| `target` | Where to copy to, in rsync's form: `user@host:/path` | |
| `delete` | Also remove files on the server that are no longer in the book | `false` |

rsync sends only what changed, so publishing a large book again is quick. It uses your SSH keys and settings as they are. The `rsync` program must be installed.
