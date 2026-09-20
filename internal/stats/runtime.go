package stats

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// CollectRuntime gathers process-level metrics. RSS is read from
// /proc/self/statm (Linux only); RSSBytes is 0 where unavailable.
func CollectRuntime(startTime time.Time) RuntimeStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return RuntimeStats{
		Uptime:         time.Since(startTime),
		RSSBytes:       int64(readRSSBytes()),
		HeapAllocBytes: int64(m.HeapAlloc),
		HeapInuseBytes: int64(m.HeapInuse),
		SysBytes:       int64(m.Sys),
		Goroutines:     int64(runtime.NumGoroutine()),
	}
}

// readRSSBytes returns resident set size in bytes from /proc/self/statm
// (second field = resident pages), or 0 on any error.
func readRSSBytes() uint64 {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	const pageSize = 4096
	return pages * pageSize
}
