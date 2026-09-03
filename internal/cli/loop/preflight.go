package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewPreflightCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preflight",
		Short: "Run pre-run checks before the execution loop starts",
		RunE:  clistub.NotImplemented("loop preflight"),
	}
}
