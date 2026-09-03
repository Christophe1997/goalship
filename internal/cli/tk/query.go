package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewQueryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "query <filter>",
		Short: "Query tickets with a jq filter",
		RunE:  clistub.NotImplemented("tk query"),
	}
}
