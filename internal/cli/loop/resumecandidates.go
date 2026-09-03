package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ledger"
)

// resumeCandidate is cmd_resume_candidates' per-run JSON shape.
type resumeCandidate struct {
	RunID               string   `json:"run_id"`
	Goal                string   `json:"goal"`
	TicketMode          *string  `json:"ticket_mode"`
	ShippedCount        int      `json:"shipped_count"`
	ConsecutiveFailures int      `json:"consecutive_failures"`
	ClaimedTicketIDs    []string `json:"claimed_ticket_ids"`
}

func NewResumeCandidatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume-candidates <repo-root>",
		Short: "List runs eligible to resume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			states, err := ledger.FindResumableRuns(args[0])
			if err != nil {
				return err
			}
			candidates := make([]resumeCandidate, 0, len(states))
			for _, s := range states {
				candidates = append(candidates, resumeCandidate{
					RunID:               s.RunID,
					Goal:                s.Goal,
					TicketMode:          s.TicketMode,
					ShippedCount:        s.ShippedCount,
					ConsecutiveFailures: s.ConsecutiveFailures,
					ClaimedTicketIDs:    nonNilStrings(s.ClaimedTicketIDs),
				})
			}
			return printJSON(cmd, candidates)
		},
	}
}
