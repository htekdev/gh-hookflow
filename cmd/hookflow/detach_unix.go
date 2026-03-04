//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setDetachAttr applies platform-specific detach attributes.
// On Unix, Setsid creates a new session so the child survives parent exit.
func setDetachAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
