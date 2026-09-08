package loop

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// runShip is cmd_ship's Go core: record the closing note (branch, PR URL,
// head SHA — mirrors reconciliation.py's record_ship_note) and close the
// ticket, both via one Load/Save round trip so no observer sees the note
// without the closed status or vice versa from this tool's own writes.
// Goes directly through internal/ticket rather than shelling out to `tk
// add-note`/`tk close` the way the Python original does.
func runShip(repoRoot, ticketID, branch, prURL, sha string) error {
	ticketsDir := filepath.Join(repoRoot, ".tickets")
	path, err := ticket.Resolve(ticketsDir, ticketID)
	if err != nil {
		return fmt.Errorf("loop ship: %w", err)
	}
	t, err := ticket.Load(path)
	if err != nil {
		return fmt.Errorf("loop ship: %w", err)
	}

	t.AddNote(fmt.Sprintf("branch: %s\npr: %s\nsha: %s", branch, prURL, sha))
	t.Status = "closed"

	if err := t.Save(path); err != nil {
		return fmt.Errorf("loop ship: %w", err)
	}
	return nil
}

func NewShipCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ship <repo-root> <ticket-id> <branch> <pr-url> <sha>",
		Short: "Record the closing note and close the ticket in one step",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShip(args[0], args[1], args[2], args[3], args[4])
		},
	}
}
