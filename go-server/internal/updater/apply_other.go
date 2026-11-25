//go:build !darwin && !linux && !windows

package updater

import (
	"fmt"
	"runtime"
)

func apply(s *Staged) error {
	return fmt.Errorf("暂不支持在 %s 上自动更新", runtime.GOOS)
}

// CleanupBackups is a no-op on unsupported platforms.
func CleanupBackups() {}
