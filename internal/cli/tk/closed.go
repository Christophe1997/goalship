package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewClosedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "closed",
		Short: "List closed tickets",
		RunE:  clistub.NotImplemented("tk closed"),
	}
}
