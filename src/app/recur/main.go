package main

import (
	"errors"
	"os"

	"github.com/directedbits/recur/src/app/recur/cli"
	displayterminal "github.com/directedbits/recur/src/infra/terminal/display"
)

func main() {
	root := cli.NewRootCmd()
	if err := root.Execute(); err != nil {
		var ambErr *displayterminal.AmbiguousError
		if errors.As(err, &ambErr) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
