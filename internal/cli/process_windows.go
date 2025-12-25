//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

// configureProcessDetachment configures the command to run detached from the parent process.
// On Windows, this sets CreationFlags to CREATE_NEW_PROCESS_GROUP to detach from the parent console.
func configureProcessDetachment(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
