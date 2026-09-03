package cli

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

// NewReviewCmd starts the browser-based ticket-graph review checkpoint for
// a run. Unlike tk/loop, it has no subcommands of its own — <run-id> is a
// direct argument to a single action.
func NewReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review <run-id>",
		Short: "Open the ticket-graph review checkpoint for a run",
		Args:  cobra.ExactArgs(1),
		RunE:  clistub.NotImplemented("review"),
	}
}

// NewReviewStatusCmd reports a run's review_state so the orchestrating
// agent can discover a pending rejection without polling the review server.
func NewReviewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review-status <run-id>",
		Short: "Report a run's review_state (pending, rejected, or approved)",
		Args:  cobra.ExactArgs(1),
		RunE:  clistub.NotImplemented("review-status"),
	}
}
