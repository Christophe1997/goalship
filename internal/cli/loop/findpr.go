package loop

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

func NewFindPRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "find-pr <repo-root> <host-tool> <branch>",
		Short: "Print the URL of branch's already-open PR/MR, if any",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if url := gitops.FindOpenPRForBranch(args[0], args[1], args[2]); url != "" {
				fmt.Fprintln(cmd.OutOrStdout(), url)
			}
			return nil
		},
	}
}
