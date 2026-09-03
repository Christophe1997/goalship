package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewReopenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reopen <ticket-id>",
		Short: "Reopen a closed ticket",
		RunE:  clistub.NotImplemented("tk reopen"),
	}
}
