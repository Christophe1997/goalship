package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link <ticket-id> <other-id>",
		Short: "Link two tickets",
		RunE:  clistub.NotImplemented("tk link"),
	}
}
