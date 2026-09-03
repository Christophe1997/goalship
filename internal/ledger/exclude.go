package ledger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ledgerDirName mirrors run_state.py's LEDGER_DIR_NAME — this package's own
// state directory inside the target repo's working tree.
const ledgerDirName = ".goalship"

// EnsureExcluded idempotently adds ledgerDirName to
// <repoRoot>/.git/info/exclude, creating the file (and its parent dirs) if
// needed. Mirrors run_state.py's ensure_ledger_excluded: excluding via
// .git/info/exclude rather than the target repo's own .gitignore keeps
// this tool's bookkeeping out of the target repo's tracked history, since
// whether to gitignore anything is the target repo's own call, not this
// tool's to make on its behalf.
//
// This runs before CommitAll's `git add`, and is why CommitAll's own
// staging pathspec never needs to name ledgerDirName explicitly the way it
// must for .tickets/: once a path is git-ignored, `git add` exits nonzero
// for any pathspec naming that path, negative or not (confirmed against a
// real repo) — so an ignored path must never appear in a pathspec at all.
func EnsureExcluded(repoRoot string) error {
	excludePath := filepath.Join(repoRoot, ".git", "info", "exclude")
	entry := "/" + ledgerDirName + "/"

	existing, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ledger: read %s: %w", excludePath, err)
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if line == entry {
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return fmt.Errorf("ledger: create %s: %w", filepath.Dir(excludePath), err)
	}
	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("ledger: open %s: %w", excludePath, err)
	}
	defer f.Close()

	var toWrite strings.Builder
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		toWrite.WriteByte('\n')
	}
	toWrite.WriteString(entry)
	toWrite.WriteByte('\n')
	if _, err := f.WriteString(toWrite.String()); err != nil {
		return fmt.Errorf("ledger: append to %s: %w", excludePath, err)
	}
	return nil
}
