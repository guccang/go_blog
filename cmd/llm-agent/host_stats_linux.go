//go:build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// getMemTotalBytes Linux: 读 /proc/meminfo
func getMemTotalBytes() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			var kb uint64
			fmt.Sscanf(line, "MemTotal: %d kB", &kb)
			return kb * 1024
		}
	}
	return 0
}
