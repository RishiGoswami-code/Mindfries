// Command mindfries is the Mindfries V2 assessment CLI.
package main

import (
	"os"

	"github.com/mindfries/v2/cli/internal/cli"
)

func main() {
	os.Exit(cli.Default().Run(os.Args[1:]))
}
