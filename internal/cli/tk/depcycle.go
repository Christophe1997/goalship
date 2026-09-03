package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func newDepCycleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cycle",
		Short: "Detect dependency cycles across all tickets",
		RunE:  clistub.NotImplemented("tk dep cycle"),
	}
}
