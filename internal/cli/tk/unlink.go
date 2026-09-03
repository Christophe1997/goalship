package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewUnlinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlink <ticket-id> <other-id>",
		Short: "Remove a link between two tickets",
		RunE:  clistub.NotImplemented("tk unlink"),
	}
}
