package performance

import (
	"runtime"
)

// MountType represents the type of filesystem mount
type MountType int

const (
	MountTypeUnknown MountType = iota
	MountTypeLocal              // Local disk (SSD, HDD)
	MountTypeNFS                // NFS
	MountTypeSMB                // SMB/CIFS
	MountTypeOther              // Other remote filesystems
)

// String returns the string representation of MountType
func (mt MountType) String() string {
	switch mt {
	case MountTypeLocal:
		return "Local"
	case MountTypeNFS:
		return "NFS"
	case MountTypeSMB:
		return "SMB/CIFS"
	case MountTypeOther:
		return "Remote"
	default:
		return "Unknown"
	}
}

// Environment holds detected environment information
type Environment struct {
	MountType  MountType
	IsRemote   bool
	FileSystem string
	Path       string
}

// SystemResources holds system resource information
type SystemResources struct {
	CPUCores       int
	TotalMemoryMB  uint64
	AvailMemoryMB  uint64
	MemoryPressure float64 // 0.0 ~ 1.0
}

// DetectEnvironment detects the filesystem environment for a given path
func DetectEnvironment(path string) (*Environment, error) {
	env := &Environment{
		Path: path,
	}

	// Platform-specific detection
	mountType, fsType, err := detectMountType(path)
	if err != nil {
		// Fallback to unknown
		env.MountType = MountTypeUnknown
		env.IsRemote = false
		env.FileSystem = "unknown"
		return env, nil
	}

	env.MountType = mountType
	env.FileSystem = fsType
	env.IsRemote = (mountType == MountTypeNFS || mountType == MountTypeSMB || mountType == MountTypeOther)

	return env, nil
}

// GetSystemResources retrieves current system resource information
func GetSystemResources() (*SystemResources, error) {
	res := &SystemResources{
		CPUCores: runtime.NumCPU(),
	}

	// Platform-specific memory detection
	totalMem, availMem, err := getMemoryInfo()
	if err != nil {
		// Fallback to reasonable defaults
		res.TotalMemoryMB = 8192  // 8GB
		res.AvailMemoryMB = 4096  // 4GB
		res.MemoryPressure = 0.5
		return res, nil
	}

	res.TotalMemoryMB = totalMem
	res.AvailMemoryMB = availMem
	
	if totalMem > 0 {
		used := totalMem - availMem
		res.MemoryPressure = float64(used) / float64(totalMem)
	}

	return res, nil
}

// DetermineOptimalWorkers determines the optimal number of workers based on environment
func DetermineOptimalWorkers(env *Environment, sysRes *SystemResources) int {
	cpuCores := sysRes.CPUCores
	
	// Base workers on environment type
	var optimalWorkers int
	
	switch env.MountType {
	case MountTypeNFS, MountTypeSMB, MountTypeOther:
		// Remote mount: more workers to utilize I/O wait time
		// 2-3x CPU cores, depending on memory pressure
		if sysRes.MemoryPressure < 0.6 {
			optimalWorkers = cpuCores * 3
		} else if sysRes.MemoryPressure < 0.8 {
			optimalWorkers = cpuCores * 2
		} else {
			optimalWorkers = cpuCores
		}
		
	case MountTypeLocal:
		// Local mount: CPU cores or slightly more
		if sysRes.MemoryPressure < 0.7 {
			optimalWorkers = cpuCores
		} else {
			optimalWorkers = cpuCores / 2
			if optimalWorkers < 1 {
				optimalWorkers = 1
			}
		}
		
	default:
		// Unknown: use conservative estimate
		optimalWorkers = cpuCores
	}
	
	// Apply constraints
	if optimalWorkers < 1 {
		optimalWorkers = 1
	}
	if optimalWorkers > cpuCores*4 {
		optimalWorkers = cpuCores * 4
	}
	
	return optimalWorkers
}
