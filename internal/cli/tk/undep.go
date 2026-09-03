package tk

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

var undepUsageErr = errors.New("Usage: ticket undep <id> <dependency-id>")

func NewUndepCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undep <ticket-id> <dep-id>",
		Short: "Remove a dependency between two tickets",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return undepUsageErr
			}
			return runUndep(cmd, args[0], args[1])
		},
	}
}

func runUndep(cmd *cobra.Command, rawID, rawDepID string) error {
	ticketsDir, err := findTicketsDir()
	if err != nil {
		return err
	}

	path, err := resolveOrBashErr(ticketsDir, rawID)
	if err != nil {
		return err
	}
	depPath, err := resolveOrBashErr(ticketsDir, rawDepID)
	if err != nil {
		return err
	}
	id, depID := idFromPath(path), idFromPath(depPath)

	t, err := ticket.Load(path)
	if err != nil {
		return err
	}

	if !containsID(t.Deps, depID) {
		// bash tk prints this to stdout and exits 1 (cmd_undep's `echo
		// "Dependency not found"; return 1`); this Go port routes the
		// same text through the normal error path instead — main.go
		// prints it to stderr — matching message and exit code, not
		// stream, to keep one error-reporting path for the whole CLI.
		// The acceptance-criteria case (a dependency ID that doesn't
		// resolve to any ticket at all) is unaffected: that's the
		// resolveOrBashErr path above, stderr in both bash and here.
		return errors.New("Dependency not found")
	}

	t.Deps = removeID(t.Deps, depID)
	if err := t.Save(path); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Removed dependency: %s -/-> %s\n", id, depID)
	return nil
}
