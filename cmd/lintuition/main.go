// Command lintuition is the lintuition CLI.
package main

import (
	"os"

	_ "github.com/ssgreg/lintuition/builtin"
	"github.com/ssgreg/lintuition/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
