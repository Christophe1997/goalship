package ticket

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Christophe1997/goalship/internal/atomicfile"
)

// RunSummaryFile marks a directory inside the tickets area as a run folder.
// It is deliberately not a .md file, so no ticket listing, lookup or bash
// tk glob ever mistakes it for a ticket.
const RunSummaryFile = "run.json"

// RunSummary is a run folder's run.json: what a repo reader needs to tell
// runs apart without the git-excluded ledger.
type RunSummary struct {
	RunID     string  `json:"run_id"`
	Goal      string  `json:"goal"`
	StartedAt string  `json:"started_at"`
	Outcome   *string `json:"outcome"`
}

// ReadRunSummary loads runDir's run.json; a missing file wraps
// os.ErrNotExist.
func ReadRunSummary(runDir string) (*RunSummary, error) {
	path := filepath.Join(runDir, RunSummaryFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ticket: read %s: %w", path, err)
	}
	return parseRunSummary(path, data)
}

func parseRunSummary(path string, data []byte) (*RunSummary, error) {
	var s RunSummary
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("ticket: parse %s (fix or delete it): %w", path, err)
	}
	return &s, nil
}

// WriteRunSummary atomically writes runDir's run.json. started_at is set
// once: an existing file's value wins over s.StartedAt. An unchanged summary
// is not rewritten, so `loop ledger` calls that only touch the ledger leave
// the tickets area alone.
func WriteRunSummary(runDir string, s RunSummary) error {
	path := filepath.Join(runDir, RunSummaryFile)
	existingData, err := os.ReadFile(path)
	switch {
	case err == nil:
		existing, err := parseRunSummary(path, existingData)
		if err != nil {
			return err
		}
		if existing.StartedAt != "" {
			s.StartedAt = existing.StartedAt
		}
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("ticket: read %s: %w", path, err)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return fmt.Errorf("ticket: encode run summary: %w", err)
	}
	if bytes.Equal(buf.Bytes(), existingData) {
		return nil
	}
	if err := atomicfile.Write(path, buf.Bytes()); err != nil {
		return fmt.Errorf("ticket: write %s: %w", path, err)
	}
	return nil
}

// ListRunDirs returns every run folder directly inside areaDir — a
// subdirectory holding run.json — sorted by name. A missing area has none.
func ListRunDirs(areaDir string) ([]string, error) {
	entries, err := os.ReadDir(areaDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ticket: scan %s: %w", areaDir, err)
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(areaDir, e.Name())
		if info, err := os.Stat(filepath.Join(dir, RunSummaryFile)); err == nil && !info.IsDir() {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}
