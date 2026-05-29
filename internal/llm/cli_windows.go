package llm

import (
	"os/exec"
	"syscall"
)

// hideConsoleWindow prevents a flashing cmd.exe window when the bot spawns a
// CLI helper from a -H windowsgui binary that has no console of its own.
func hideConsoleWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
