//go:build linux

package performance

import (
	"syscall"
)

// Linux filesystem type magic numbers
const (
	NFS_SUPER_MAGIC    = 0x6969
	SMB_SUPER_MAGIC    = 0x517B
	CIFS_MAGIC_NUMBER  = 0xFF534D42
	EXT4_SUPER_MAGIC   = 0xEF53
	XFS_SUPER_MAGIC    = 0x58465342
	BTRFS_SUPER_MAGIC  = 0x9123683E
)

// detectMountType detects the mount type for a given path on Linux
func detectMountType(path string) (MountType, string, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return MountTypeUnknown, "", err
	}

	// Check filesystem type based on magic number
	fsType := ""
	mountType := MountTypeUnknown

	switch stat.Type {
	case NFS_SUPER_MAGIC:
		fsType = "nfs"
		mountType = MountTypeNFS
	case SMB_SUPER_MAGIC, CIFS_MAGIC_NUMBER:
		fsType = "cifs"
		mountType = MountTypeSMB
	case EXT4_SUPER_MAGIC:
		fsType = "ext4"
		mountType = MountTypeLocal
	case XFS_SUPER_MAGIC:
		fsType = "xfs"
		mountType = MountTypeLocal
	case BTRFS_SUPER_MAGIC:
		fsType = "btrfs"
		mountType = MountTypeLocal
	default:
		// Unknown type, assume local if common values
		if stat.Type > 0 && stat.Type < 0x10000 {
			fsType = "unknown-local"
			mountType = MountTypeLocal
		} else {
			fsType = "unknown-remote"
			mountType = MountTypeOther
		}
	}

	return mountType, fsType, nil
}

// getMemoryInfo retrieves memory information on Linux
func getMemoryInfo() (totalMB, availMB uint64, err error) {
	var info syscall.Sysinfo_t
	err = syscall.Sysinfo(&info)
	if err != nil {
		return 0, 0, err
	}

	// Convert from bytes to MB
	unit := uint64(info.Unit)
	totalMB = info.Totalram * unit / 1024 / 1024
	availMB = info.Freeram * unit / 1024 / 1024

	return totalMB, availMB, nil
}
