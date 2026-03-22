package tasks

import (
	"os/exec"
)

// findExecutable looks for an executable in PATH.
func findExecutable(name string) (string, error) {
	return exec.LookPath(name)
}

// newCommand creates an exec.Cmd with stderr/stdout suppressed.
func newCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	return cmd
}
