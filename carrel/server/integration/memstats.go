//go:build integration

package integration

import "runtime"

func heapAllocBytes() uint64 {
	runtime.GC()
	var s runtime.MemStats
	runtime.ReadMemStats(&s)
	return s.Alloc
}
