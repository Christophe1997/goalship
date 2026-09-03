package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewCreatePRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create-pr",
		Short: "Open a pull request for the current branch",
		RunE:  clistub.NotImplemented("loop create-pr"),
	}
}
