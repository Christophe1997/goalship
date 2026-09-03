package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewPushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push",
		Short: "Push the current branch",
		RunE:  clistub.NotImplemented("loop push"),
	}
}
