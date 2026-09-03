package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

func NewResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset <repo-root> <base-ref>",
		Short: "Hard-reset the working tree to base-ref, excluding .tickets/",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return gitops.Reset(args[0], args[1])
		},
	}
}
