package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewUndepCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undep <ticket-id> <dep-id>",
		Short: "Remove a dependency between two tickets",
		RunE:  clistub.NotImplemented("tk undep"),
	}
}
