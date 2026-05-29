package main

import (
	"context"
	"fmt"
	"os"

	"github.com/AnandChowdhary/gitclaw/internal/gitclaw"
)

func main() {
	if err := gitclaw.RunCLI(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gitclaw:", err)
		os.Exit(1)
	}
}
