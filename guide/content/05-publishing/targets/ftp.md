# FTP

For a web host that takes uploads over FTP, as many shared hosts do.

```yaml
publish:
  ftp:
    host: ftp.example.com
    user: me
    path: public_html/book
```

```sh
export OCTAVO_FTP_PASSWORD='...'
octavo publish ftp
```

| Setting | Meaning | Default |
|---|---|---|
| `host` | The server | |
| `port` | The port | `21` |
| `user` | Your user name | |
| `path` | The folder on the server | |
| `tls` | `explicit` (FTPS), `implicit`, or `none` (plain FTP) | `explicit` |
| `password-env` | The environment variable that holds the password | `OCTAVO_FTP_PASSWORD` |

The password is never written in `octavo.yml`, so the settings file can go into git safely.

> [!CAUTION]
> `tls: none` sends your password unencrypted. Use it only when the host offers nothing else.

Every file is uploaded, replacing what is there. Files on the server that are no longer part of the book are not deleted.
