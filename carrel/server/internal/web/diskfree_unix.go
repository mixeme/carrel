//go:build !windows

package web

import "syscall"

// diskFree is the space left for this process in dir's file system.
func diskFree(dir string) (uint64, bool) {
	var st syscall.Statfs_t
	if syscall.Statfs(dir, &st) != nil {
		return 0, false
	}
	return uint64(st.Bavail) * uint64(st.Bsize), true
}
