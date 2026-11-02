// Command tabdl is the entry point for the tabdl coding-agent CLI.
package main

import (
	"os"

	"github.com/amir/tabdl/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
