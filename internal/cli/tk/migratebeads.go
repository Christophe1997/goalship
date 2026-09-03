package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewMigrateBeadsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate-beads <path>",
		Short: "Migrate a .beads/issues.jsonl file into .tickets/",
		RunE:  clistub.NotImplemented("tk migrate-beads"),
	}
}
