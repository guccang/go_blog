//go:build darwin || linux

package main

import (
	"fmt"
	"runtime"
	"syscall"
)

// collectHostStats 采集主机资源快照（CPU/内存/磁盘）
func collectHostStats(workspace string) map[string]any {
	stats := map[string]any{
		"cpu_cores": runtime.NumCPU(),
	}

	if memBytes := getMemTotalBytes(); memBytes > 0 {
		stats["mem_total_gb"] = fmt.Sprintf("%.1f", float64(memBytes)/(1024*1024*1024))
	}

	diskPath := workspace
	if diskPath == "" {
		diskPath = "/"
	}
	if totalGB, freeGB := getDiskStats(diskPath); totalGB > 0 {
		stats["disk_total_gb"] = fmt.Sprintf("%.1f", totalGB)
		stats["disk_free_gb"] = fmt.Sprintf("%.1f", freeGB)
	}

	return stats
}

// getDiskStats 获取指定路径所在分区的磁盘总量和可用量（GB）
func getDiskStats(path string) (totalGB, freeGB float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	bsize := uint64(stat.Bsize)
	totalGB = float64(stat.Blocks*bsize) / (1024 * 1024 * 1024)
	freeGB = float64(stat.Bavail*bsize) / (1024 * 1024 * 1024)
	return
}
