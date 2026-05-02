//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func configureProcessTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil && pgid > 0 {
		if killErr := syscall.Kill(-pgid, syscall.SIGKILL); killErr != nil && killErr != syscall.ESRCH {
			return killErr
		}
		return nil
	}
	if pid > 0 {
		// 父进程可能已被 CommandContext 结束；Setpgid=true 时进程组 ID 仍等于父 PID。
		if killErr := syscall.Kill(-pid, syscall.SIGKILL); killErr == nil || killErr == syscall.ESRCH {
			return nil
		}
		if killErr := syscall.Kill(pid, syscall.SIGKILL); killErr == nil || killErr == syscall.ESRCH {
			return nil
		} else {
			return killErr
		}
	}
	return nil
}
