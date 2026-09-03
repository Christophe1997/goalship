package tk

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// runShow is cmd_show's Go core: the target file's raw content (with its
// "parent:" line annotated by title, when the parent id is known) plus
// derived Blockers/Blocking/Children/Linked sections. bash tk pipes this
// through $TICKET_PAGER when stdout is a tty; that interactive nicety is
// out of scope here — output always goes to w.
func runShow(ticketsDir, id string, w io.Writer) error {
	path, err := ticket.Resolve(ticketsDir, id)
	if err != nil {
		return fmt.Errorf("tk show: %w", err)
	}
	targetID := strings.TrimSuffix(filepath.Base(path), ".md")

	target, err := ticket.Load(path)
	if err != nil {
		return fmt.Errorf("tk show: %w", err)
	}

	infos, err := loadTicketInfos(ticketsDir)
	if err != nil {
		return fmt.Errorf("tk show: %w", err)
	}
	byID := make(map[string]ticketInfo, len(infos))
	for _, in := range infos {
		byID[in.id] = in
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("tk show: %w", err)
	}
	if err := writeAnnotatedFile(w, raw, byID); err != nil {
		return fmt.Errorf("tk show: %w", err)
	}

	writeRelationSection(w, "Blockers", blockerLines(target.Deps, byID))
	writeRelationSection(w, "Blocking", blockingLines(targetID, infos, byID))
	writeRelationSection(w, "Children", childrenLines(targetID, infos, byID))
	writeRelationSection(w, "Linked", linkedLines(target.Links, byID))

	return nil
}

// writeAnnotatedFile re-emits raw line by line (matching bash tk's getline
// loop — always \n-terminated regardless of the source's own trailing
// newline), appending "  # <title>" to the frontmatter's parent: line only
// when that parent id is a known ticket.
func writeAnnotatedFile(w io.Writer, raw []byte, byID map[string]ticketInfo) error {
	inFront := false
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 10<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			inFront = !inFront
			fmt.Fprintln(w, line)
			continue
		}
		if inFront && strings.HasPrefix(line, "parent:") {
			p := strings.TrimLeft(strings.TrimPrefix(line, "parent:"), " ")
			if pInfo, ok := byID[p]; ok {
				fmt.Fprintf(w, "%s  # %s\n", line, pInfo.title)
				continue
			}
		}
		fmt.Fprintln(w, line)
	}
	return scanner.Err()
}

func writeRelationSection(w io.Writer, heading string, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(w, "\n## %s\n\n", heading)
	for _, l := range lines {
		fmt.Fprint(w, l)
	}
}

// blockerLines walks the target's own deps in file order — a dangling dep
// id misses byID and reads back as a zero-value ticketInfo (empty status,
// never "closed"), so it still appears here with blank status/title
// rather than being dropped; this is the `show` analogue of the ready/
// blocked dangling-dep behavior, intentionally left as-is.
func blockerLines(deps []string, byID map[string]ticketInfo) []string {
	var lines []string
	for _, dep := range deps {
		in := byID[dep]
		if in.status != "closed" {
			lines = append(lines, fmt.Sprintf("- %s [%s] %s\n", dep, in.status, in.title))
		}
	}
	return lines
}

// blockingLines and childrenLines scan every other ticket, whose
// directory-order is not itself meaningful here (bash tk's own awk
// iterates an associative array in unspecified order); sorting by id
// gives deterministic output instead of replicating that non-determinism.
func blockingLines(targetID string, infos []ticketInfo, byID map[string]ticketInfo) []string {
	var ids []string
	for _, in := range infos {
		if in.status == "closed" {
			continue
		}
		for _, d := range in.deps {
			if d == targetID {
				ids = append(ids, in.id)
				break
			}
		}
	}
	sort.Strings(ids)
	lines := make([]string, len(ids))
	for i, id := range ids {
		in := byID[id]
		lines[i] = fmt.Sprintf("- %s [%s] %s\n", in.id, in.status, in.title)
	}
	return lines
}

func childrenLines(targetID string, infos []ticketInfo, byID map[string]ticketInfo) []string {
	var ids []string
	for _, in := range infos {
		if in.parent == targetID {
			ids = append(ids, in.id)
		}
	}
	sort.Strings(ids)
	lines := make([]string, len(ids))
	for i, id := range ids {
		in := byID[id]
		lines[i] = fmt.Sprintf("- %s [%s] %s\n", in.id, in.status, in.title)
	}
	return lines
}

func linkedLines(links []string, byID map[string]ticketInfo) []string {
	lines := make([]string, len(links))
	for i, l := range links {
		in := byID[l]
		lines[i] = fmt.Sprintf("- %s [%s] %s\n", l, in.status, in.title)
	}
	return lines
}

func NewShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <ticket-id>",
		Short: "Display a ticket, including derived relationship sections",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := resolveTicketsDir(false)
			if err != nil {
				return err
			}
			return runShow(ticketsDir, args[0], cmd.OutOrStdout())
		},
	}
}
