package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewClaimCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "claim <ticket-id>",
		Short: "Claim a ticket for the current run",
		RunE:  clistub.NotImplemented("loop claim"),
	}
}
