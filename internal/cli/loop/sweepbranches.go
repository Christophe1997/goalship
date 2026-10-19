package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
	"github.com/Christophe1997/goalship/internal/runfolder"
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
			// Every run folder, not one run's: sweep may only delete
			// branches some ticket's notes name.
			ticketsDirs, err := runfolder.All(args[0])
			if err != nil {
				return err
			}
			candidates, err := gitops.SweepBranches(args[0], ticketsDirs, args[1], execute)
			if err != nil {
				return err
			}
			return printJSON(cmd, sweepCandidatesJSON(candidates))
		},
	}

	cmd.Flags().BoolVar(&execute, "delete", false, "Actually delete eligible branches from origin instead of just reporting them")

	return cmd
}
