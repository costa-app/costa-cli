package main

import (
	"os"

	"github.com/costa-app/costa-cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
