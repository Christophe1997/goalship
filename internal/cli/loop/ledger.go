package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewLedgerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ledger",
		Short: "Read or update the run ledger",
		RunE:  clistub.NotImplemented("loop ledger"),
	}
}
