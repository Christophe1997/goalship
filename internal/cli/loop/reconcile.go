package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewReconcileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reconcile",
		Short: "Reconcile the run ledger against repo state",
		RunE:  clistub.NotImplemented("loop reconcile"),
	}
}
