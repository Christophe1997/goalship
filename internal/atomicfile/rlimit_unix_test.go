//go:build !windows

package atomicfile

import "syscall"

const rlimitSupported = true

// setFileSizeLimit caps the calling process's max file size at n bytes; the
// kernel delivers SIGXFSZ (default action: terminate) the moment a write
// would cross it — a deterministic stand-in for "the process died mid-write
// at some byte offset," used only by the crash-simulation test.
func setFileSizeLimit(n uint64) error {
	return syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: n, Max: n})
}
