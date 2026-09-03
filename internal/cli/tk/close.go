package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewCloseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "close <ticket-id>",
		Short: "Close a ticket",
		RunE:  clistub.NotImplemented("tk close"),
	}
}
