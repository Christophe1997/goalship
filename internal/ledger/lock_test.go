package ledger

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAcquireReviewLock_SecondAcquisitionFailsFast(t *testing.T) {
	dir := t.TempDir()
	runID := "run-a"

	first, err := AcquireReviewLock(dir, runID)
	if err != nil {
		t.Fatalf("first AcquireReviewLock: %v", err)
	}
	defer first.Release()

	_, err = AcquireReviewLock(dir, runID)
	if err == nil {
		t.Fatal("second AcquireReviewLock: expected error, got nil")
	}
	var held *LockHeldError
	if !errors.As(err, &held) {
		t.Fatalf("second AcquireReviewLock error = %v (%T), want *LockHeldError", err, err)
	}
	if held.RunID != runID {
		t.Errorf("LockHeldError.RunID = %q, want %q", held.RunID, runID)
	}
	wantPath := filepath.Join(dir, ".goalship", runID+".review.lock")
	if held.Path != wantPath {
		t.Errorf("LockHeldError.Path = %q, want %q", held.Path, wantPath)
	}
	if !strings.Contains(err.Error(), runID) {
		t.Errorf("error message %q does not name the run id %q", err.Error(), runID)
	}
}

func TestAcquireReviewLock_DifferentRunIDsDoNotContend(t *testing.T) {
	dir := t.TempDir()

	a, err := AcquireReviewLock(dir, "run-a")
	if err != nil {
		t.Fatalf("AcquireReviewLock(run-a): %v", err)
	}
	defer a.Release()

	b, err := AcquireReviewLock(dir, "run-b")
	if err != nil {
		t.Fatalf("AcquireReviewLock(run-b): %v", err)
	}
	defer b.Release()
}

func TestAcquireReviewLock_ReleaseAllowsReacquisition(t *testing.T) {
	dir := t.TempDir()
	runID := "run-a"

	lock, err := AcquireReviewLock(dir, runID)
	if err != nil {
		t.Fatalf("AcquireReviewLock: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	again, err := AcquireReviewLock(dir, runID)
	if err != nil {
		t.Fatalf("AcquireReviewLock after Release: %v", err)
	}
	defer again.Release()
}

// TestAcquireReviewLock_SurvivesDiscardedHandle guards against os.File's
// close-on-finalize behavior: if the package didn't root acquired locks
// itself, a caller that discards the *ReviewLock (as e.g. a fire-and-forget
// caller might) would have its lock silently released by the GC while the
// process is still very much alive, breaking "held for the process's
// lifetime" without any error or signal.
func TestAcquireReviewLock_SurvivesDiscardedHandle(t *testing.T) {
	dir := t.TempDir()
	if _, err := AcquireReviewLock(dir, "run-gc"); err != nil {
		t.Fatalf("AcquireReviewLock: %v", err)
	}

	runtime.GC()
	runtime.GC()

	if _, err := AcquireReviewLock(dir, "run-gc"); err == nil {
		t.Fatal("lock was released after its handle became unreachable")
	}
}

// lockHolderRepoRootEnv and lockHolderRunIDEnv, when set, tell this test
// binary it was re-exec'd as TestAcquireReviewLock_KillReleasesLock's child:
// acquire the lock, announce it on stdout, then block forever so the parent
// can kill it uncleanly.
const lockHolderRepoRootEnv = "LEDGER_LOCKHOLDER_REPOROOT"
const lockHolderRunIDEnv = "LEDGER_LOCKHOLDER_RUNID"

// TestAcquireReviewLock_KillReleasesLock proves the actual invariant this
// package exists for: the kernel, not application logic, releases the lock
// when its holder dies. An in-process goroutine holding the lock and simply
// stopping would never exercise the OS's release-on-close-of-fd behavior —
// only a real process death does, so this spawns and kills one.
func TestAcquireReviewLock_KillReleasesLock(t *testing.T) {
	if repoRoot := os.Getenv(lockHolderRepoRootEnv); repoRoot != "" {
		runID := os.Getenv(lockHolderRunIDEnv)
		if _, err := AcquireReviewLock(repoRoot, runID); err != nil {
			fmt.Fprintf(os.Stderr, "lockholder child: AcquireReviewLock: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("locked")
		select {}
	}

	dir := t.TempDir()
	runID := "run-kill"

	cmd := exec.Command(os.Args[0], "-test.run=^TestAcquireReviewLock_KillReleasesLock$")
	cmd.Env = append(os.Environ(),
		lockHolderRepoRootEnv+"="+dir,
		lockHolderRunIDEnv+"="+runID,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Covers every failure path below (including t.Fatal, which exits via
	// goroutine panic/runtime.Goexit) so a still-running child never
	// outlives the test as an orphan.
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	ready := make(chan error, 1)
	go func() {
		s := bufio.NewScanner(stdout)
		if s.Scan() && s.Text() == "locked" {
			ready <- nil
			return
		}
		ready <- fmt.Errorf("child did not report lock acquisition (scan err: %v, stderr: %s)", s.Err(), stderr.String())
	}()

	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for child to acquire the lock")
	}

	if _, err := AcquireReviewLock(dir, runID); err == nil {
		t.Fatal("AcquireReviewLock succeeded while the child still holds the lock")
	} else if !errors.As(err, new(*LockHeldError)) {
		t.Fatalf("AcquireReviewLock error while child holds lock = %v, want *LockHeldError", err)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	_ = cmd.Wait()

	lock, err := AcquireReviewLock(dir, runID)
	if err != nil {
		t.Fatalf("AcquireReviewLock after killing the holder: %v", err)
	}
	defer lock.Release()
}
