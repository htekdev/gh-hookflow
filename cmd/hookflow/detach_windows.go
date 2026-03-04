//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// setDetachAttr applies platform-specific detach attributes.
// On Windows, CREATE_NO_WINDOW fully detaches and suppresses the console window.
// DETACHED_PROCESS is NOT used alongside CREATE_NO_WINDOW (they conflict per MSDN).
func setDetachAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
