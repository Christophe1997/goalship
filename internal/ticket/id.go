package ticket

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotFound is returned by Resolve when no ticket file matches id.
var ErrNotFound = errors.New("ticket: not found")

// ErrAmbiguous is returned by Resolve when id matches more than one ticket
// file and no exact match disambiguates it.
var ErrAmbiguous = errors.New("ticket: ambiguous id")

// Resolve finds the ticket file matching id within ticketsDir: an exact
// "<id>.md" filename match, else a single unambiguous substring match
// anywhere in .tickets/*.md filenames — mirrors bash tk's ticket_path().
// Matching is against the full filename (extension included, as tk's own
// `find -name "*${id}*.md"` does), not just the ID portion in isolation.
func Resolve(ticketsDir, id string) (string, error) {
	// tk's own ticket_path() trims argv whitespace first ("handles
	// Claude/agent quirks", its comment says) — callers here are the same
	// agent-driven kind.
	id = strings.TrimSpace(id)

	exact := filepath.Join(ticketsDir, id+".md")
	if info, err := os.Stat(exact); err == nil && !info.IsDir() {
		return exact, nil
	}

	// tk's find ... 2>/dev/null swallows a missing/unreadable ticketsDir
	// into a plain "0 matches"; mirror that rather than surfacing the raw
	// stat/permission error, so callers can errors.Is(err, ErrNotFound)
	// uniformly regardless of cause.
	entries, err := os.ReadDir(ticketsDir)
	if err != nil {
		return "", fmt.Errorf("%w: %q in %s: %v", ErrNotFound, id, ticketsDir, err)
	}

	var matches []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		if strings.Contains(name, id) {
			matches = append(matches, name)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%w: %q in %s", ErrNotFound, id, ticketsDir)
	case 1:
		return filepath.Join(ticketsDir, matches[0]), nil
	default:
		return "", fmt.Errorf("%w: %q matches %s", ErrAmbiguous, id, strings.Join(matches, ", "))
	}
}

const idSuffixAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// GenerateID produces a new "<repo-prefix>-<YYYYMMDD-HHMM>-<4-char>" ID
// (R4): a readable date-time segment — so a directory listing sorts in
// creation order — plus tk's existing 4-char random-suffix convention,
// retried on the rare collision with an existing file. repo-prefix
// derivation matches bash tk's generate_id(): ticketsDir's parent
// directory name stands in for tk's basename(pwd), which assumes
// ticketsDir sits at the repo root — true the way tk (and this package's
// callers) are normally invoked.
func GenerateID(ticketsDir string) (string, error) {
	prefix, err := repoPrefix(ticketsDir)
	if err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102-1504")

	const maxAttempts = 5
	for i := 0; i < maxAttempts; i++ {
		suffix, err := randomSuffix(4)
		if err != nil {
			return "", fmt.Errorf("ticket: generate id: %w", err)
		}
		id := fmt.Sprintf("%s-%s-%s", prefix, stamp, suffix)
		if _, err := os.Stat(filepath.Join(ticketsDir, id+".md")); os.IsNotExist(err) {
			return id, nil
		}
	}
	return "", fmt.Errorf("ticket: generate id: no unused suffix after %d attempts", maxAttempts)
}

// repoPrefix mirrors bash tk's generate_id(): split the directory name on
// '-'/'_', take each segment's first byte; if that's under 2 chars (a
// single unbroken segment), fall back to the directory name's first 3
// bytes. Byte-based, matching generate_id()'s awk substr/sed pipeline
// under a typical ASCII repo directory name.
func repoPrefix(ticketsDir string) (string, error) {
	abs, err := filepath.Abs(ticketsDir)
	if err != nil {
		return "", fmt.Errorf("ticket: repo prefix: %w", err)
	}
	dirName := filepath.Base(filepath.Dir(abs))

	segments := strings.FieldsFunc(dirName, func(r rune) bool {
		return r == '-' || r == '_'
	})
	var b strings.Builder
	for _, seg := range segments {
		b.WriteByte(seg[0])
	}
	if prefix := b.String(); len(prefix) >= 2 {
		return prefix, nil
	}
	if len(dirName) < 3 {
		return dirName, nil
	}
	return dirName[:3], nil
}

// randomSuffix returns n random lowercase alphanumeric characters, matching
// tk's `LC_ALL=C tr -dc 'a-z0-9' < /dev/urandom | head -c n`. Rejection
// sampling below 252 (= 7*36, the largest multiple of 36 under 256) keeps
// the distribution over the 36-char alphabet unbiased.
func randomSuffix(n int) (string, error) {
	const alphabetLen = byte(len(idSuffixAlphabet))
	const rejectAt = 7 * alphabetLen

	buf := make([]byte, n)
	var b [1]byte
	for i := range buf {
		for {
			if _, err := rand.Read(b[:]); err != nil {
				return "", err
			}
			if b[0] < rejectAt {
				buf[i] = idSuffixAlphabet[b[0]%alphabetLen]
				break
			}
		}
	}
	return string(buf), nil
}
