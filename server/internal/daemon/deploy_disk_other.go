//go:build !linux && !darwin && !windows

package daemon

import "errors"

func deploymentFreeBytes(directory string) (*uint64, error) {
	return nil, errors.New("disk capacity probe is unsupported on this platform")
}
