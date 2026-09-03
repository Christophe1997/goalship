// Package tk implements goalship's ticket CRUD/graph/query surface — a Go
// port of bash tk (wedow/ticket) invoked as `goalship tk <subcommand>`.
package tk

import "github.com/spf13/cobra"

// NewCmd returns the "tk" parent command with every ticket subcommand
// attached. The caller (internal/cli/root.go) assigns its cobra.Group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tk",
		Short: "Ticket CRUD, dependency graph, and query commands",
	}

	cmd.AddCommand(
		NewCreateCmd(),
		NewStartCmd(),
		NewCloseCmd(),
		NewReopenCmd(),
		NewStatusCmd(),
		NewDepCmd(),
		NewUndepCmd(),
		NewLinkCmd(),
		NewUnlinkCmd(),
		NewLsCmd(),
		NewReadyCmd(),
		NewBlockedCmd(),
		NewClosedCmd(),
		NewShowCmd(),
		NewEditCmd(),
		NewAddNoteCmd(),
		NewQueryCmd(),
		NewMigrateBeadsCmd(),
	)

	return cmd
}
