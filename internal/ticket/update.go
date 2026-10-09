package ticket

import (
	"path/filepath"
	"strings"
)

// Update resolves id within ticketsDir, loads that ticket strictly, applies
// mutate, and saves it — the one resolve-load-mutate-save sequence every
// single-ticket writer shares, so a caller that needs several changes (a
// note and a status) makes them all in one save. It returns the resolved
// ticket ID, which may be fuller than the id the caller passed. On any
// error the file is left as it was.
func Update(ticketsDir, id string, mutate func(*Ticket)) (string, error) {
	path, err := Resolve(ticketsDir, id)
	if err != nil {
		return "", err
	}
	t, err := Load(path)
	if err != nil {
		return "", err
	}
	mutate(t)
	if err := t.Save(path); err != nil {
		return "", err
	}
	return strings.TrimSuffix(filepath.Base(path), ".md"), nil
}
