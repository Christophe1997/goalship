package tk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// ticketsDirEnv mirrors bash tk's TICKETS_DIR override, checked before the
// parent-directory walk.
const ticketsDirEnv = "TICKETS_DIR"

// locateTicketsDir mirrors bash tk's find_tickets_dir(): the TICKETS_DIR
// env var if set — returned as-is, with no existence check, exactly like
// bash's own `[[ -n "${TICKETS_DIR:-}" ]] && { echo "$TICKETS_DIR";
// return 0; }` — else the nearest ".tickets" directory found by walking
// up from the current directory (checking "/" last, exactly once,
// regardless of whether cwd itself is "/"). Whether the result must
// exist is the caller's decision (init_tickets_dir's own
// is_write_cmd branch): findTicketsDir requires it, findOrInitTicketsDir
// doesn't.
func locateTicketsDir() (dir string, ok bool) {
	if v := os.Getenv(ticketsDirEnv); v != "" {
		return v, true
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for dir := cwd; dir != "/"; dir = filepath.Dir(dir) {
		if candidate := filepath.Join(dir, ".tickets"); isDir(candidate) {
			return candidate, true
		}
	}
	if isDir("/.tickets") {
		return "/.tickets", true
	}
	return "", false
}

// findTicketsDir is locateTicketsDir for a read command: the located
// directory must actually exist.
func findTicketsDir() (string, error) {
	dir, ok := locateTicketsDir()
	if !ok {
		return "", errors.New("Error: no .tickets directory found (searched parent directories)\n" +
			"Run 'tk create' to initialize, or set TICKETS_DIR env var")
	}
	if !isDir(dir) {
		return "", fmt.Errorf("Error: tickets directory '%s' does not exist", dir)
	}
	return dir, nil
}

// findOrInitTicketsDir is locateTicketsDir for a WRITE_COMMANDS command
// (migrate-beads is the only one in this ticket's scope): falls back to
// a fresh "./.tickets" when nothing is found, and — like bash's
// init_tickets_dir for a write command — never errors just because the
// located directory doesn't exist yet; the caller creates it.
func findOrInitTicketsDir() (string, error) {
	dir, ok := locateTicketsDir()
	if !ok {
		return ".tickets", nil
	}
	return dir, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// idFromPath returns a resolved ticket's ID from its file path — bash
// tk's ubiquitous `basename "$file" .md`.
func idFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".md")
}

// resolveOrBashErr wraps ticket.Resolve, reformatting ErrNotFound/
// ErrAmbiguous into bash tk's own ticket_path() message text (stderr,
// via the returned error — main.go prints it) so callers match exactly.
func resolveOrBashErr(ticketsDir, id string) (string, error) {
	path, err := ticket.Resolve(ticketsDir, id)
	if err == nil {
		return path, nil
	}
	trimmed := strings.TrimSpace(id)
	switch {
	case errors.Is(err, ticket.ErrAmbiguous):
		return "", fmt.Errorf("Error: ambiguous ID '%s' matches multiple tickets", trimmed)
	case errors.Is(err, ticket.ErrNotFound):
		return "", fmt.Errorf("Error: ticket '%s' not found", trimmed)
	default:
		return "", err
	}
}

// containsID reports whether target is already present in ids — used
// for deps/links membership checks. bash tk instead greps the raw
// "[a, b]" field text for the substring target, so a target that is
// itself a substring of another entry reads as "already present" in
// bash but not here; real tk IDs never nest as substrings of each other
// in practice, so this exact-match version is the sane behavior for the
// one case that matters (avoiding a duplicate dep/link).
func containsID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

// removeID returns ids with every occurrence of target removed,
// preserving relative order — bash's sed removal is also global (`/g`).
func removeID(ids []string, target string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != target {
			out = append(out, id)
		}
	}
	return out
}
