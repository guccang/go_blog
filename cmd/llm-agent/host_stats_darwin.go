//go:build darwin

package main

import (
	"encoding/binary"
	"syscall"
)

// getMemTotalBytes macOS: sysctl hw.memsize
func getMemTotalBytes() uint64 {
	val, err := syscall.Sysctl("hw.memsize")
	if err != nil || len(val) < 8 {
		return 0
	}
	return binary.LittleEndian.Uint64([]byte(val[:8]))
}
