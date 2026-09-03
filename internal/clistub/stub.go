// Package clistub gives every not-yet-implemented subcommand the same RunE
// body, so later tickets replace one stub function per command without
// touching how the command tree itself is wired.
package clistub

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NotImplemented returns a RunE for a command whose real behavior a later
// ticket fills in. name is the full invocation, e.g. "tk create" or
// "loop resolve-base", used in the error text.
func NotImplemented(name string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("%s: not yet implemented", name)
	}
}
