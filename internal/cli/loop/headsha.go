package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewHeadSHACmd() *cobra.Command {
	return &cobra.Command{
		Use:   "head-sha",
		Short: "Print the current HEAD commit SHA",
		RunE:  clistub.NotImplemented("loop head-sha"),
	}
}
