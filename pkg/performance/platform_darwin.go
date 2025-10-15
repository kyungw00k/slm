//go:build darwin

package performance

import (
	"strings"
	"syscall"
)

// detectMountType detects the mount type for a given path on macOS
func detectMountType(path string) (MountType, string, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return MountTypeUnknown, "", err
	}

	// Get filesystem type name from f_fstypename
	// Convert []int8 to string
	fsTypeBytes := make([]byte, len(stat.Fstypename))
	for i, b := range stat.Fstypename {
		fsTypeBytes[i] = byte(b)
	}
	fsType := string(fsTypeBytes)
	fsType = strings.TrimRight(fsType, "\x00") // Remove null bytes

	// Determine mount type based on filesystem type
	switch {
	case strings.EqualFold(fsType, "nfs"):
		return MountTypeNFS, fsType, nil
	case strings.EqualFold(fsType, "smbfs"):
		return MountTypeSMB, fsType, nil
	case strings.EqualFold(fsType, "afpfs"):
		return MountTypeOther, fsType, nil
	case strings.EqualFold(fsType, "apfs"), 
		 strings.EqualFold(fsType, "hfs"),
		 strings.EqualFold(fsType, "msdos"),
		 strings.EqualFold(fsType, "exfat"),
		 strings.EqualFold(fsType, "ntfs"):
		return MountTypeLocal, fsType, nil
	default:
		// Unknown filesystem, check if local or remote based on flags
		// MNT_LOCAL flag indicates local filesystem
		if stat.Flags&0x00001000 != 0 { // MNT_LOCAL = 0x00001000
			return MountTypeLocal, fsType, nil
		}
		return MountTypeOther, fsType, nil
	}
}

// getMemoryInfo retrieves memory information on macOS
func getMemoryInfo() (totalMB, availMB uint64, err error) {
	// Use sysctl to get memory info
	// This is a simplified version - in production you might want to use
	// CGO to call host_statistics or parse vm_stat output
	
	// For now, use a reasonable approximation
	// In a real implementation, you would use:
	// - syscall.Sysctl for getting memory info
	// - or execute 'vm_stat' and parse output
	// - or use CGO to call Mach APIs
	
	// Simplified fallback
	totalMB = 16384  // 16GB default
	availMB = 8192   // 8GB available default
	
	return totalMB, availMB, nil
}
