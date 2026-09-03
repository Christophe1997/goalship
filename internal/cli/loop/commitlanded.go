package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewCommitLandedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit-landed <ticket-id>",
		Short: "Report whether a ticket's commit landed on the base branch",
		RunE:  clistub.NotImplemented("loop commit-landed"),
	}
}
