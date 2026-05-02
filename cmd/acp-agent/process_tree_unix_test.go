//go:build !windows

package main

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestKillProcessTreeKillsChildAfterParentExit(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30 >/dev/null 2>&1 & echo $!")
	configureProcessTree(cmd)

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("start parent process: %v", err)
	}

	childPID, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("parse child pid from %q: %v", string(out), err)
	}
	defer syscall.Kill(childPID, syscall.SIGKILL)

	if err := syscall.Kill(childPID, 0); err != nil {
		t.Fatalf("child process is not alive before cleanup: %v", err)
	}

	if err := killProcessTree(cmd); err != nil {
		t.Fatalf("killProcessTree: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(childPID, 0); err == syscall.ESRCH {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child process %d still exists after process tree cleanup", childPID)
}
