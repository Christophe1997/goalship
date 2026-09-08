package loop

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

func NewHeadSHACmd() *cobra.Command {
	return &cobra.Command{
		Use:   "head-sha <repo-root> <branch>",
		Short: "Print branch's head commit SHA",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sha, err := gitops.HeadSHA(args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), sha)
			return nil
		},
	}
}
