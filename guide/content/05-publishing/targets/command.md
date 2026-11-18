# Any command

For a host with its own upload tool, run that tool. `{dir}` is replaced with the folder of the built book:

```yaml
publish:
  cloudflare:
    type: command
    run: npx wrangler pages deploy {dir} --project-name my-book
```

More examples:

```yaml
publish:
  netlify:
    type: command
    run: netlify deploy --prod --dir {dir}
  s3:
    type: command
    run: aws s3 sync {dir} s3://my-bucket/book --delete
```

| Setting | Meaning |
|---|---|
| `run` | The command line. It runs in the book's folder, through `sh`. |

The command also gets the built folder in the `OCTAVO_SITE_DIR` environment variable, for scripts.
