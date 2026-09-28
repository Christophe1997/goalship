package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

// mergeOrderEntryJSON is one land-ordered open PR's wire shape.
type mergeOrderEntryJSON struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
	Base   string `json:"base"`
}

func mergeOrderEntriesJSON(prs []gitops.OpenPR) []mergeOrderEntryJSON {
	out := make([]mergeOrderEntryJSON, 0, len(prs)) // [] not null when empty
	for _, pr := range prs {
		out = append(out, mergeOrderEntryJSON{
			Number: pr.Number,
			URL:    pr.URL,
			Branch: pr.Branch,
			Base:   pr.Base,
		})
	}
	return out
}

func NewMergeOrderCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "merge-order <repo-root> <host-tool>",
		Short: "Print every open PR/MR's topological land order as JSON",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			prs, err := gitops.ListOpenPRs(args[0], args[1])
			if err != nil {
				return err
			}
			order, err := gitops.TopologicalMergeOrder(prs)
			if err != nil {
				return err
			}
			return printJSON(cmd, mergeOrderEntriesJSON(order))
		},
	}
}
