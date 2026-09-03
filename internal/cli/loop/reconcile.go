package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

// reconcileActionJSON is one ReconciliationAction's wire shape — snake_case
// keys matching loop_runner.py's cmd_reconcile output exactly.
type reconcileActionJSON struct {
	TicketID string `json:"ticket_id"`
	Outcome  string `json:"outcome"`
	Detail   string `json:"detail"`
	PRRef    string `json:"pr_ref"`
}

// reconcileResultJSON is loop reconcile's wire shape, matching
// loop_runner.py's cmd_reconcile: {"actions": [...], "auth_failure": ...}.
type reconcileResultJSON struct {
	Actions     []reconcileActionJSON `json:"actions"`
	AuthFailure *string               `json:"auth_failure"`
}

func reconcileActionsJSON(actions []gitops.ReconciliationAction) []reconcileActionJSON {
	out := make([]reconcileActionJSON, 0, len(actions)) // [] not null when empty
	for _, a := range actions {
		out = append(out, reconcileActionJSON{
			TicketID: a.TicketID,
			Outcome:  a.Outcome,
			Detail:   a.Detail,
			PRRef:    a.PRRef,
		})
	}
	return out
}

func NewReconcileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reconcile <repo-root>",
		Short: "Cross-check every in-progress ticket against git/PR state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := gitops.Reconcile(args[0])
			if err != nil {
				return err
			}
			return printJSON(cmd, reconcileResultJSON{
				Actions:     reconcileActionsJSON(report.Actions),
				AuthFailure: strPtrOrNil(report.AuthFailure),
			})
		},
	}
}
