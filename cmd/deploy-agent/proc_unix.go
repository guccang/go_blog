//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr 设置 Setpgid=true 使子进程脱离当前进程组
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
