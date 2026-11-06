package system

import "syscall"

type DiskSpace struct {
	Total uint64
	Free  uint64
}

func DiskSpaceAt(path string) (DiskSpace, error) {
	total, _, free, err := statDisk(path)
	if err != nil {
		return DiskSpace{}, err
	}
	return DiskSpace{Total: total, Free: free}, nil
}

func statDisk(path string) (total, used, free uint64, err error) {
	var stat syscall.Statfs_t
	if err = syscall.Statfs(path, &stat); err != nil {
		return
	}

	total = stat.Blocks * uint64(stat.Bsize)
	free = stat.Bfree * uint64(stat.Bsize)
	used = total - free
	return
}
