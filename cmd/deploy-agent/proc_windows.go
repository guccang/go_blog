//go:build windows

package main

import "os/exec"

// setSysProcAttr Windows 上无需设置进程组属性
func setSysProcAttr(cmd *exec.Cmd) {
	// Windows 不支持 Setpgid，无需额外设置
}
