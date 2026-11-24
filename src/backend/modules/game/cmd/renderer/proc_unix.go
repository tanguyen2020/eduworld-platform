//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// isolate starts Godot in its own process group and kills the whole group on cancel, so a timeout also stops the
// children of the wrapper (xvfb-run, Xvfb, Godot).
func isolate(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
