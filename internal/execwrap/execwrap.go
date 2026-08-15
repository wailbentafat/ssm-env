// Package execwrap replaces the current process with a target command
// after loading secrets into its environment, so secrets exist only in
// process memory and are never printed to stdout or passed through a
// shell.
package execwrap

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Run loads secrets into the process environment via os.Setenv, then
// replaces the current process image with args[0] (resolved via PATH),
// passing args as its argv. On success it never returns -- the calling
// process becomes the target command, same PID.
func Run(args []string, secrets map[string]string) error {
	if len(args) == 0 {
		return fmt.Errorf("exec mode requires a command after --")
	}

	for key, value := range secrets {
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("setting %s: %w", key, err)
		}
	}

	binary, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("command not found: %s", args[0])
	}

	return syscall.Exec(binary, args, os.Environ())
}
