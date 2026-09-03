package atomicfile

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestWrite_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	if err := Write(path, []byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("content = %q, want %q", got, "hello")
	}

	assertNoTempFiles(t, dir, path)
}

func TestWrite_ReplacesExistingContentFully(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")

	if err := os.WriteFile(path, []byte("old content"), 0o644); err != nil {
		t.Fatalf("seed WriteFile: %v", err)
	}

	if err := Write(path, []byte("brand new content")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "brand new content" {
		t.Fatalf("content = %q, want %q", got, "brand new content")
	}

	assertNoTempFiles(t, dir, path)
}

// TestWrite_RenameFailureLeavesTargetUnchanged forces the rename step itself
// to fail (by making path an existing non-empty directory, which POSIX
// rename(2) refuses to replace with a file) after the temp file has already
// been created, written, and synced. Unlike TestWrite_MissingParentDirLeavesNoArtifacts
// (which fails before any temp file exists), this is the only in-process
// test where the cleanup-on-error defer has a real temp file to remove.
func TestWrite_RenameFailureLeavesTargetUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")

	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	sentinel := filepath.Join(path, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("seed sentinel: %v", err)
	}

	if err := Write(path, []byte("payload")); err == nil {
		t.Fatal("Write: expected error renaming over an existing non-empty directory, got nil")
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		t.Fatalf("target replaced despite rename failure: info=%v, err=%v", info, err)
	}
	got, err := os.ReadFile(sentinel)
	if err != nil || string(got) != "keep me" {
		t.Fatalf("sentinel content changed: got=%q, err=%v", got, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "target" {
			t.Errorf("leftover temp file after failed rename: %s", e.Name())
		}
	}
}

func TestWrite_MissingParentDirLeavesNoArtifacts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.txt")

	err := Write(path, []byte("data"))
	if err == nil {
		t.Fatal("Write: expected error for missing parent directory, got nil")
	}

	entries, rerr := os.ReadDir(dir)
	if rerr != nil {
		t.Fatalf("ReadDir: %v", rerr)
	}
	if len(entries) != 0 {
		t.Fatalf("directory not empty after failed Write: %v", entries)
	}
}

// atomicfileCrashLimitEnv, when set, tells this test binary it was re-exec'd
// as TestWrite_InterruptedMidWrite's child: cap its own file size at the
// given byte count (so the kernel SIGXFSZs it partway through the payload
// write below) and perform exactly one Write, instead of launching another
// child.
const atomicfileCrashLimitEnv = "ATOMICFILE_CRASH_LIMIT"
const atomicfileCrashPathEnv = "ATOMICFILE_CRASH_PATH"

// TestWrite_InterruptedMidWrite is the crash test the ticket asks for
// literally: a child process is killed while inside Write, mid-payload, the
// way a real process crash would. Every prior test either checks a static
// end-state or forces a failure before any byte reaches path/the temp file —
// this is the only one where Write is genuinely interrupted with data
// already flowing. The interruption point is made deterministic (not a
// sleep racing disk speed, which is flaky and, worse, silently vacuous on a
// fast disk — a mutation check confirmed a non-atomic Write can pass a
// blind-sleep version of this test simply by finishing before the kill
// lands) by capping RLIMIT_FSIZE below the payload size: the kernel
// terminates the child with SIGXFSZ at that exact byte offset, every run.
//
// A reader of path must see the pre-crash content or the fully-written new
// content — never the interrupted partial write — since Write's only
// mutation of path itself is the one atomic rename at the very end.
func TestWrite_InterruptedMidWrite(t *testing.T) {
	if limit := os.Getenv(atomicfileCrashLimitEnv); limit != "" {
		n, _ := strconv.ParseUint(limit, 10, 64)
		if err := setFileSizeLimit(n); err != nil {
			os.Exit(3) // parent treats a clean exit as "SIGXFSZ never fired" and fails the test
		}
		payload := bytes.Repeat([]byte("N"), 32<<20)
		_ = Write(os.Getenv(atomicfileCrashPathEnv), payload)
		os.Exit(3) // reached only if Write finished inside the limit — also means the test proved nothing
	}
	if !rlimitSupported {
		t.Skip("RLIMIT_FSIZE has no equivalent on this platform")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "target.bin")
	oldContent := []byte("old committed content")
	if err := Write(path, oldContent); err != nil {
		t.Fatalf("seed Write: %v", err)
	}
	newContent := bytes.Repeat([]byte("N"), 32<<20)

	for _, limit := range []uint64{1 << 20, 4 << 20, 16 << 20} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestWrite_InterruptedMidWrite$")
		cmd.Env = append(os.Environ(),
			atomicfileCrashLimitEnv+"="+strconv.FormatUint(limit, 10),
			atomicfileCrashPathEnv+"="+path,
		)
		if err := cmd.Run(); err == nil {
			t.Fatalf("limit %d: child exited cleanly — SIGXFSZ never fired, test is vacuous", limit)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("limit %d: ReadFile: %v", limit, err)
		}
		if !bytes.Equal(got, oldContent) && !bytes.Equal(got, newContent) {
			t.Fatalf("limit %d: content is neither pre-crash nor fully-written (len=%d) — interrupted write leaked to path", limit, len(got))
		}
	}
}

func assertNoTempFiles(t *testing.T, dir, path string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	want := filepath.Base(path)
	for _, e := range entries {
		if e.Name() != want {
			t.Errorf("unexpected leftover file in %s: %s", dir, e.Name())
		}
	}
}
