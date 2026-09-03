package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewFindPRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "find-pr <branch>",
		Short: "Find an open pull request for a branch",
		RunE:  clistub.NotImplemented("loop find-pr"),
	}
}
