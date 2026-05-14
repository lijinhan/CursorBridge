//go:build windows

package updater

import (
	"syscall"
)

func detachedProcessAttr() (*syscall.SysProcAttr, error) {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}, nil
}
