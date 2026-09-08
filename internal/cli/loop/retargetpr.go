package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

func NewRetargetPRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retarget-pr <repo-root> <host-tool> <pr-ref> <new-base>",
		Short: "Change an already-open PR/MR's base branch",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			return gitops.RetargetPullRequest(args[0], args[1], args[2], args[3])
		},
	}
}
