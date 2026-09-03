package tk

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveTicketsDir mirrors bash tk's find_tickets_dir/init_tickets_dir:
// TICKETS_DIR wins outright, else walk up from cwd for the nearest
// .tickets directory. allowCreate mirrors tk's WRITE_COMMANDS list
// (currently just "create" on the Go side): those commands may proceed
// with a not-yet-existing "./.tickets" instead of erroring.
func resolveTicketsDir(allowCreate bool) (string, error) {
	if dir := os.Getenv("TICKETS_DIR"); dir != "" {
		return dir, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("tk: %w", err)
	}
	for dir := cwd; ; {
		candidate := filepath.Join(dir, ".tickets")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if allowCreate {
		return ".tickets", nil
	}
	return "", fmt.Errorf("no .tickets directory found (searched parent directories); run 'tk create' to initialize, or set TICKETS_DIR")
}
