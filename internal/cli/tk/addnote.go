package tk

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// notesHeadingPresent mirrors bash tk's `grep -q '^## Notes'` (a prefix
// match, not the trailing-\s*$ anchor reconciliation.py's own heading
// regex uses — this only decides whether to insert a fresh heading).
var notesHeadingPresent = regexp.MustCompile(`(?m)^## Notes`)

// runAddNote is cmd_add_note's Go core: append a "## Notes" heading (if
// absent) then a "**<timestamp>**\n\n<note>\n" block. The exact spacing
// here (single "\n## Notes\n", no trailing blank line) is load-bearing:
// it's what keeps the block parseable by reconciliation.py's
// _NOTES_HEADING_RE/_NOTE_MARKER_RE (see addnote_test.go).
func runAddNote(ticketsDir, id, note string) (string, error) {
	path, err := ticket.Resolve(ticketsDir, id)
	if err != nil {
		return "", fmt.Errorf("tk add-note: %w", err)
	}
	t, err := ticket.Load(path)
	if err != nil {
		return "", fmt.Errorf("tk add-note: %w", err)
	}

	if !notesHeadingPresent.MatchString(t.Body) {
		t.Body += "\n## Notes\n"
	}
	t.Body += fmt.Sprintf("\n**%s**\n\n%s\n", isoNow(), note)

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
