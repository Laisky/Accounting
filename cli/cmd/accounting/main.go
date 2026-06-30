// Package main starts the Accounting command line client.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Laisky/Accounting/cli/internal/app"
)

// main executes the command line client and exits with a non-zero status on failure.
func main() {
	if err := app.Run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "accounting: %v\n", err)
		os.Exit(1)
	}
}
