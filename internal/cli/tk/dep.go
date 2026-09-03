package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewDepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep <ticket-id> <dep-id>",
		Short: "Add a dependency between two tickets",
		RunE:  clistub.NotImplemented("tk dep"),
	}
	cmd.AddCommand(newDepTreeCmd(), newDepCycleCmd())
	return cmd
}
