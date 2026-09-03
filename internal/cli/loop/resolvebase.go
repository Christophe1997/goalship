package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewResolveBaseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resolve-base",
		Short: "Resolve the trunk base branch",
		RunE:  clistub.NotImplemented("loop resolve-base"),
	}
}
