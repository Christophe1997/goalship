package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewRetargetPRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retarget-pr <pr-number>",
		Short: "Retarget an open pull request's base branch",
		RunE:  clistub.NotImplemented("loop retarget-pr"),
	}
}
