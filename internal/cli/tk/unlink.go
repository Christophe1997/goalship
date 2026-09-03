package tk

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

var unlinkUsageErr = errors.New("Usage: ticket unlink <id> <target-id>")

func NewUnlinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlink <ticket-id> <target-id>",
		Short: "Remove a link between two tickets",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return unlinkUsageErr
			}
			return runUnlink(cmd, args[0], args[1])
		},
	}
}

func runUnlink(cmd *cobra.Command, rawID, rawTargetID string) error {
	ticketsDir, err := findTicketsDir()
	if err != nil {
		return err
	}

	path, err := resolveOrBashErr(ticketsDir, rawID)
	if err != nil {
		return err
	}
	targetPath, err := resolveOrBashErr(ticketsDir, rawTargetID)
	if err != nil {
		return err
	}
	id, targetID := idFromPath(path), idFromPath(targetPath)

	t, err := ticket.Load(path)
	if err != nil {
		return err
	}
	if !containsID(t.Links, targetID) {
		// Same stdout->stderr stream simplification as undep's
		// "Dependency not found" (see undep.go); the acceptance-criteria
		// nonexistent-ID case is the resolveOrBashErr path above, which
		// matches bash's stream exactly.
		return errors.New("Link not found")
	}

	t.Links = removeID(t.Links, targetID)
	if err := t.Save(path); err != nil {
		return err
	}

	// The reverse direction is unconditional, matching
	// remove_link_from_file: a no-op if the target's own Links doesn't
	// have id, never an error.
	tt, err := ticket.Load(targetPath)
	if err != nil {
		return err
	}
	tt.Links = removeID(tt.Links, id)
	if err := tt.Save(targetPath); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed link: %s <-> %s\n", id, targetID)
	return nil
}
