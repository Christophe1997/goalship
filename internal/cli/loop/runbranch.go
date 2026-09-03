package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewRunBranchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run-branch <run-id>",
		Short: "Compute the branch name for a run",
		RunE:  clistub.NotImplemented("loop run-branch"),
	}
}
