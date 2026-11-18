# Error handling

Functions that can fail return `Result`. The `?` operator passes an error up to the caller.

```rust
use std::fs;
use std::io;

fn read_title(path: &str) -> Result<String, io::Error> {
    let text = fs::read_to_string(path)?;
    Ok(text.lines().next().unwrap_or("").to_string())
}
```

> [!CAUTION]
> `unwrap()` panics on an error. It is fine in examples and tests, and a bug waiting to happen in a server.

See [ownership](../01-foundations/02-ownership.md) for why `text` can be returned safely.
