# Channels

A channel carries values from one goroutine to another.

```go
results := make(chan int)

go func() {
	defer close(results)
	for i := range 3 {
		results <- i * i
	}
}()

for r := range results {
	fmt.Println(r)
}
```

> [!TIP]
> The sender closes the channel. The receiver's `range` loop ends when it is closed and empty.
