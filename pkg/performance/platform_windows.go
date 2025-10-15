//go:build windows

package performance

import (
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getDriveTypeW      = kernel32.NewProc("GetDriveTypeW")
	globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

// Windows drive types
const (
	DRIVE_UNKNOWN     = 0
	DRIVE_NO_ROOT_DIR = 1
	DRIVE_REMOVABLE   = 2
	DRIVE_FIXED       = 3
	DRIVE_REMOTE      = 4
	DRIVE_CDROM       = 5
	DRIVE_RAMDISK     = 6
)

// detectMountType detects the mount type for a given path on Windows
func detectMountType(path string) (MountType, string, error) {
	// Get the root path (e.g., C:\)
	rootPath := path
	if len(path) >= 2 && path[1] == ':' {
		rootPath = path[:2] + "\\"
	}

	// Convert to UTF16
	pathPtr, err := syscall.UTF16PtrFromString(rootPath)
	if err != nil {
		return MountTypeUnknown, "", err
	}

	// Call GetDriveTypeW
	ret, _, _ := getDriveTypeW.Call(uintptr(unsafe.Pointer(pathPtr)))
	driveType := uint32(ret)

	var mountType MountType
	var fsType string

	switch driveType {
	case DRIVE_FIXED:
		mountType = MountTypeLocal
		fsType = "NTFS/FAT32"
	case DRIVE_REMOTE:
		// Could be SMB or other network drive
		if strings.HasPrefix(path, "\\\\") {
			mountType = MountTypeSMB
			fsType = "SMB"
		} else {
			mountType = MountTypeOther
			fsType = "Network"
		}
	case DRIVE_REMOVABLE:
		mountType = MountTypeLocal
		fsType = "Removable"
	case DRIVE_CDROM:
		mountType = MountTypeLocal
		fsType = "CD-ROM"
	case DRIVE_RAMDISK:
		mountType = MountTypeLocal
		fsType = "RAMDisk"
	default:
		mountType = MountTypeUnknown
		fsType = "Unknown"
	}

	return mountType, fsType, nil
}

// MEMORYSTATUSEX structure for Windows
type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

// getMemoryInfo retrieves memory information on Windows
func getMemoryInfo() (totalMB, availMB uint64, err error) {
	var memStatus memoryStatusEx
	memStatus.dwLength = uint32(unsafe.Sizeof(memStatus))

	ret, _, err := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret == 0 {
		return 0, 0, err
	}

	// Convert from bytes to MB
	totalMB = memStatus.ullTotalPhys / 1024 / 1024
	availMB = memStatus.ullAvailPhys / 1024 / 1024

	return totalMB, availMB, nil
}
