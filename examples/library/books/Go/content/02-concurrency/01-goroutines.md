# Goroutines

A goroutine is a function running on its own. Start one with `go`.

```go
var wg sync.WaitGroup
for i := range 3 {
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("worker", i)
	}()
}
wg.Wait()
```

> [!WARNING]
> `main` does not wait for goroutines. Without `wg.Wait()` the program may exit before any worker prints.

Goroutines talk to each other through [channels](02-channels.md).
