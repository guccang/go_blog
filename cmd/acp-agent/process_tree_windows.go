//go:build windows

package main

import "os/exec"

func configureProcessTree(cmd *exec.Cmd) {
}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
