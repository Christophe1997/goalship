package main

import (
	"fmt"
	"os"

	"github.com/Christophe1997/goalship/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
