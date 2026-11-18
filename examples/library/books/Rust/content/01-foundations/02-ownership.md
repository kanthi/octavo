# Ownership

Every value has one owner. When the owner goes out of scope, the value is freed.

```rust
let a = String::from("book");
let b = a;            // ownership moves to b
// println!("{a}");   // error: a was moved
println!("{b}");
```

## Borrowing

A reference borrows a value without taking it.

```rust
fn length(s: &str) -> usize {
    s.len()
}

let title = String::from("Boring Rust");
println!("{}", length(&title));   // title is still usable
```

> [!IMPORTANT]
> At any time you can have either one mutable reference or any number of shared references. Never both.
