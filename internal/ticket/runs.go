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
	"github.com/Christophe1997/goalship/internal/ledger"
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

// RunDir returns runID's folder inside areaDir. The folder name is
// sanitized exactly as the run's ledger file name is, and a name that
// would escape or alias the area itself is refused.
func RunDir(areaDir, runID string) (string, error) {
	safe := ledger.SafeRunID(runID)
	if safe == "" || safe == "." || safe == ".." {
		return "", fmt.Errorf("ticket: invalid run id %q", runID)
	}
	return filepath.Join(areaDir, safe), nil
}

// ReadRunSummary loads runDir's run.json; a missing file wraps
// os.ErrNotExist.
func ReadRunSummary(runDir string) (*RunSummary, error) {
	path := filepath.Join(runDir, RunSummaryFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ticket: read %s: %w", path, err)
	}
	var s RunSummary
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("ticket: parse %s: %w", path, err)
	}
	return &s, nil
}

// WriteRunSummary atomically writes runDir's run.json. started_at is set
// once: an existing file's value wins over s.StartedAt.
func WriteRunSummary(runDir string, s RunSummary) error {
	existing, err := ReadRunSummary(runDir)
	switch {
	case err == nil && existing.StartedAt != "":
		s.StartedAt = existing.StartedAt
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return err
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return fmt.Errorf("ticket: encode run summary: %w", err)
	}
	path := filepath.Join(runDir, RunSummaryFile)
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
