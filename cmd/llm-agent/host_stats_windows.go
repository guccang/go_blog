//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceEx   = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
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
		diskPath = "C:\\"
	}
	if totalGB, freeGB := getDiskStats(diskPath); totalGB > 0 {
		stats["disk_total_gb"] = fmt.Sprintf("%.1f", totalGB)
		stats["disk_free_gb"] = fmt.Sprintf("%.1f", freeGB)
	}

	return stats
}

// getDiskStats 获取指定路径所在分区的磁盘总量和可用量（GB）
func getDiskStats(path string) (totalGB, freeGB float64) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0
	}
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	ret, _, _ := procGetDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 {
		return 0, 0
	}
	totalGB = float64(totalBytes) / (1024 * 1024 * 1024)
	freeGB = float64(freeBytesAvailable) / (1024 * 1024 * 1024)
	return
}

// getMemTotalBytes 获取物理内存总量
func getMemTotalBytes() uint64 {
	// MEMORYSTATUSEX 结构体大小为 64 字节
	var memInfo [64]byte
	*(*uint32)(unsafe.Pointer(&memInfo[0])) = uint32(len(memInfo))
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memInfo[0])))
	if ret == 0 {
		return 0
	}
	// ullTotalPhys 位于偏移 8 处（uint64）
	return *(*uint64)(unsafe.Pointer(&memInfo[8]))
}
