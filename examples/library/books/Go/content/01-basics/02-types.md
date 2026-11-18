# Types

Go is statically typed. `:=` declares a variable and takes its type from the value.

```go
count := 3            // int
ratio := 0.75         // float64
name := "gopher"      // string
var ready bool        // false: the zero value
```

## Zero values

| Type | Zero value |
|---|---|
| `int`, `float64` | `0` |
| `string` | `""` |
| `bool` | `false` |
| pointer, slice, map | `nil` |

## Structs

```go
type Book struct {
	Title    string
	Chapters int
}

b := Book{Title: "Go", Chapters: 4}
```
