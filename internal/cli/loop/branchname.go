package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewBranchNameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "branch-name <ticket-id>",
		Short: "Compute the branch name for a ticket",
		RunE:  clistub.NotImplemented("loop branch-name"),
	}
}
