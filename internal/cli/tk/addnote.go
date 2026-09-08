package tk

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// runAddNote is cmd_add_note's Go core: resolve id and delegate the
// heading/timestamp formatting to ticket.Ticket.AddNote (shared with `loop
// ship`'s closing note) before saving.
func runAddNote(ticketsDir, id, note string) (string, error) {
	path, err := ticket.Resolve(ticketsDir, id)
	if err != nil {
		return "", fmt.Errorf("tk add-note: %w", err)
	}
	t, err := ticket.Load(path)
	if err != nil {
		return "", fmt.Errorf("tk add-note: %w", err)
	}

	t.AddNote(note)

	if err := t.Save(path); err != nil {
		return "", fmt.Errorf("tk add-note: %w", err)
	}
	return strings.TrimSuffix(filepath.Base(path), ".md"), nil
}

func NewAddNoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-note <ticket-id> [text...]",
		Short: "Append a timestamped note to a ticket (or pipe text via stdin)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var note string
			switch {
			case len(args) > 1:
				note = strings.Join(args[1:], " ")
			case !stdinIsTTY():
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("tk add-note: reading stdin: %w", err)
				}
				// $(cat) strips every trailing newline from a command
				// substitution; TrimRight matches that exactly.
				note = strings.TrimRight(string(data), "\n")
			default:
				return fmt.Errorf("tk add-note: no note provided")
			}

			ticketsDir, err := findTicketsDir()
			if err != nil {
				return err
			}
			resolvedID, err := runAddNote(ticketsDir, args[0], note)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Note added to %s\n", resolvedID)
			return nil
		},
	}
}
