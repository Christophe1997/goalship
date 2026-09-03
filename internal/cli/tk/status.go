package tk

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// validStatuses is bash tk's VALID_STATUSES.
var validStatuses = map[string]bool{"open": true, "in_progress": true, "closed": true}

// runStatus is cmd_status's Go core: resolve id, validate the transition,
// and persist it. Returns the resolved ticket id (for the "Updated <id> ->
// <status>" message), matching bash tk printing the resolved basename
// rather than the caller's (possibly partial) id argument.
func runStatus(ticketsDir, id, status string) (string, error) {
	if !validStatuses[status] {
		return "", fmt.Errorf("tk status: invalid status %q (must be one of open, in_progress, closed)", status)
	}
	path, err := ticket.Resolve(ticketsDir, id)
	if err != nil {
		return "", fmt.Errorf("tk status: %w", err)
	}
	t, err := ticket.Load(path)
	if err != nil {
		return "", fmt.Errorf("tk status: %w", err)
	}
	t.Status = status
	if err := t.Save(path); err != nil {
		return "", fmt.Errorf("tk status: %w", err)
	}
	return strings.TrimSuffix(filepath.Base(path), ".md"), nil
}

func reportStatusChange(cmd *cobra.Command, ticketsDir, id, status string) error {
	resolvedID, err := runStatus(ticketsDir, id, status)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Updated %s -> %s\n", resolvedID, status)
	return nil
}

func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <ticket-id> <status>",
		Short: "Update a ticket's status (open|in_progress|closed)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := resolveTicketsDir(false)
			if err != nil {
				return err
			}
			return reportStatusChange(cmd, ticketsDir, args[0], args[1])
		},
	}
}
