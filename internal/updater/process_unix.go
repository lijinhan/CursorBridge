//go:build !windows

package updater

import (
	"syscall"
)

func detachedProcessAttr() (*syscall.SysProcAttr, error) {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}, nil
}
