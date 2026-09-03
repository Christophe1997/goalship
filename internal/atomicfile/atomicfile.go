// Package atomicfile writes files so a concurrent reader never observes a
// partially-written result: content lands fully or the previous file stays
// visible, even across a process crash between the write and the rename.
package atomicfile

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// Write replaces path with data, atomically. It writes data to a temp file
// beside path, fsyncs it, then renames it over path — POSIX guarantees a
// rename to an existing name is atomic only within the same filesystem, so
// the temp file must live in path's own directory, not a shared os.TempDir.
func Write(path string, data []byte) (err error) {
	tmpPath, err := tempPath(path)
	if err != nil {
		return fmt.Errorf("atomicfile: %w", err)
	}

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("atomicfile: create temp file: %w", err)
	}
	defer func() {
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	if _, werr := f.Write(data); werr != nil {
		f.Close()
		err = fmt.Errorf("atomicfile: write temp file: %w", werr)
		return err
	}
	if serr := f.Sync(); serr != nil {
		f.Close()
		err = fmt.Errorf("atomicfile: sync temp file: %w", serr)
		return err
	}
	if cerr := f.Close(); cerr != nil {
		err = fmt.Errorf("atomicfile: close temp file: %w", cerr)
		return err
	}

	if rerr := os.Rename(tmpPath, path); rerr != nil {
		err = fmt.Errorf("atomicfile: rename temp file into place: %w", rerr)
		return err
	}
	return nil
}

func tempPath(path string) (string, error) {
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("generate temp suffix: %w", err)
	}
	name := filepath.Base(path) + "." + hex.EncodeToString(suffix) + ".tmp"
	return filepath.Join(filepath.Dir(path), name), nil
}
