package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

func NewPushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push <repo-root> <branch-name>",
		Short: "Push branch-name to origin, tracked (never force)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return gitops.PushBranch(args[0], args[1])
		},
	}
}
