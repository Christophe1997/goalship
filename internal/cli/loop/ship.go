package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewShipCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ship <ticket-id>",
		Short: "Ship a completed ticket end-to-end",
		RunE:  clistub.NotImplemented("loop ship"),
	}
}
