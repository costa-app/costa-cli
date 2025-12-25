//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

// configureProcessDetachment configures the command to run detached from the parent process.
// On Unix systems, this sets Setpgid to create a new process group.
func configureProcessDetachment(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
