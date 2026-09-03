package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewDirtyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dirty",
		Short: "Report whether the working tree has unexpected dirty paths",
		RunE:  clistub.NotImplemented("loop dirty"),
	}
}
