// Command octavo builds and publishes Markdown books.
package main

import (
	"os"

	"github.com/kanthi/octavo/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
