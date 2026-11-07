package daemon

import "golang.org/x/sys/windows"

func deploymentFreeBytes(directory string) (*uint64, error) {
	path, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return nil, err
	}
	var available uint64
	if err := windows.GetDiskFreeSpaceEx(path, &available, nil, nil); err != nil {
		return nil, err
	}
	return &available, nil
}
