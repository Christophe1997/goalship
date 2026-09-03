//go:build windows

package atomicfile

import "errors"

const rlimitSupported = false

func setFileSizeLimit(n uint64) error {
	return errors.New("RLIMIT_FSIZE has no Windows equivalent")
}
