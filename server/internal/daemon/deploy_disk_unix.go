//go:build linux || darwin

package daemon

import "golang.org/x/sys/unix"

func deploymentFreeBytes(directory string) (*uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(directory, &stat); err != nil {
		return nil, err
	}
	available := uint64(stat.Bavail) * uint64(stat.Bsize)
	return &available, nil
}
