package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset <run-id>",
		Short: "Reset a run's ledger state",
		RunE:  clistub.NotImplemented("loop reset"),
	}
}
