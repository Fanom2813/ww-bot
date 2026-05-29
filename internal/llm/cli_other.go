//go:build !windows

package llm

import "os/exec"

// hideConsoleWindow is a no-op on non-Windows platforms — only cmd.exe pops up
// a visible console for headless GUI parents.
func hideConsoleWindow(_ *exec.Cmd) {}
