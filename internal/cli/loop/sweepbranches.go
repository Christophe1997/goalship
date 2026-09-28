package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

// sweepCandidateJSON is one SweepCandidate's wire shape — snake_case keys.
type sweepCandidateJSON struct {
	TicketID string  `json:"ticket_id"`
	Branch   string  `json:"branch"`
	PRRef    string  `json:"pr_ref"`
	Outcome  string  `json:"outcome"`
	Error    *string `json:"error"`
}

func sweepCandidatesJSON(candidates []gitops.SweepCandidate) []sweepCandidateJSON {
	out := make([]sweepCandidateJSON, 0, len(candidates)) // [] not null when empty
	for _, c := range candidates {
		out = append(out, sweepCandidateJSON{
			TicketID: c.TicketID,
			Branch:   c.Branch,
			PRRef:    c.PRRef,
			Outcome:  c.Outcome,
			Error:    strPtrOrNil(c.Error),
		})
	}
	return out
}

func NewSweepBranchesCmd() *cobra.Command {
	var execute bool

	cmd := &cobra.Command{
		Use:   "sweep-branches <repo-root> <host-tool>",
		Short: "Report or delete merged goalship-managed branches no open PR still references",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			candidates, err := gitops.SweepBranches(args[0], args[1], execute)
			if err != nil {
				return err
			}
			return printJSON(cmd, sweepCandidatesJSON(candidates))
		},
	}

	cmd.Flags().BoolVar(&execute, "delete", false, "Actually delete eligible branches from origin instead of just reporting them")

	return cmd
}
