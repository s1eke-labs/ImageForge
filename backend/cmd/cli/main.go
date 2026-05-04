package main

import (
	"os"

	"imageforge/backend/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		os.Exit(1)
	}
}
