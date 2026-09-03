package tk

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

var linkUsageErr = errors.New("Usage: ticket link <id> <id> [id...]")

func NewLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link <id> <id> [id...]",
		Short: "Link tickets together (symmetric)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return linkUsageErr
			}
			return runLink(cmd, args)
		},
	}
}

func runLink(cmd *cobra.Command, rawIDs []string) error {
	ticketsDir, err := findTicketsDir()
	if err != nil {
		return err
	}

	paths := make([]string, len(rawIDs))
	ids := make([]string, len(rawIDs))
	for i, raw := range rawIDs {
		path, err := resolveOrBashErr(ticketsDir, raw)
		if err != nil {
			return err
		}
		paths[i] = path
		ids[i] = idFromPath(path)
	}

	count := 0
	for i, path := range paths {
		t, err := ticket.Load(path)
		if err != nil {
			return err
		}
		changed := false
		for j, other := range ids {
			if i == j {
				continue
			}
			if !containsID(t.Links, other) {
				t.Links = append(t.Links, other)
				count++
				changed = true
			}
		}
		if changed {
			if err := t.Save(path); err != nil {
				return err
			}
		}
	}

	if count == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "All links already exist")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Added %d link(s) between %d tickets\n", count, len(ids))
	}
	return nil
}
