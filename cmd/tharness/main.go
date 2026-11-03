// Command tharness is the entry point for the tharness coding-agent CLI.
package main

import (
	"os"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
