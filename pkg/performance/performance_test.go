package performance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectEnvironment(t *testing.T) {
	env, err := DetectEnvironment("/tmp")
	require.NoError(t, err)
	require.NotNil(t, env)

	assert.NotEmpty(t, env.FileSystem)
	assert.NotEmpty(t, env.Path)
	assert.Contains(t, []MountType{MountTypeUnknown, MountTypeLocal, MountTypeNFS, MountTypeSMB, MountTypeOther}, env.MountType)
}

func TestMountType_String(t *testing.T) {
	tests := []struct {
		mountType MountType
		expected  string
	}{
		{MountTypeLocal, "Local"},
		{MountTypeNFS, "NFS"},
		{MountTypeSMB, "SMB/CIFS"},
		{MountTypeOther, "Remote"},
		{MountTypeUnknown, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mountType.String())
		})
	}
}

func TestGetSystemResources(t *testing.T) {
	res, err := GetSystemResources()
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Greater(t, res.CPUCores, 0)
	assert.Greater(t, res.TotalMemoryMB, uint64(0))
	assert.GreaterOrEqual(t, res.AvailMemoryMB, uint64(0))
	assert.GreaterOrEqual(t, res.MemoryPressure, 0.0)
	assert.LessOrEqual(t, res.MemoryPressure, 1.0)
}

func TestDetermineOptimalWorkers(t *testing.T) {
	tests := []struct {
		name      string
		env       *Environment
		sysRes    *SystemResources
		minExpected int
		maxExpected int
	}{
		{
			name: "local filesystem low memory pressure",
			env: &Environment{
				MountType: MountTypeLocal,
				IsRemote:  false,
			},
			sysRes: &SystemResources{
				CPUCores:       8,
				MemoryPressure: 0.5,
			},
			minExpected: 8,
			maxExpected: 8,
		},
		{
			name: "local filesystem high memory pressure",
			env: &Environment{
				MountType: MountTypeLocal,
				IsRemote:  false,
			},
			sysRes: &SystemResources{
				CPUCores:       8,
				MemoryPressure: 0.8,
			},
			minExpected: 4,
			maxExpected: 4,
		},
		{
			name: "nfs remote mount low memory pressure",
			env: &Environment{
				MountType: MountTypeNFS,
				IsRemote:  true,
			},
			sysRes: &SystemResources{
				CPUCores:       8,
				MemoryPressure: 0.5,
			},
			minExpected: 24,
			maxExpected: 24,
		},
		{
			name: "nfs remote mount high memory pressure",
			env: &Environment{
				MountType: MountTypeNFS,
				IsRemote:  true,
			},
			sysRes: &SystemResources{
				CPUCores:       8,
				MemoryPressure: 0.9,
			},
			minExpected: 8,
			maxExpected: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineOptimalWorkers(tt.env, tt.sysRes)
			assert.GreaterOrEqual(t, result, tt.minExpected)
			assert.LessOrEqual(t, result, tt.maxExpected)
			assert.GreaterOrEqual(t, result, 1) // Should always be at least 1
		})
	}
}

func TestDetermineOptimalWorkers_Constraints(t *testing.T) {
	// Test that workers are constrained to reasonable limits
	env := &Environment{
		MountType: MountTypeNFS,
		IsRemote:  true,
	}

	sysRes := &SystemResources{
		CPUCores:       32, // High CPU count
		MemoryPressure: 0.1, // Low memory pressure
	}

	result := DetermineOptimalWorkers(env, sysRes)

	// Should not exceed 4x CPU cores
	assert.LessOrEqual(t, result, 32*4)
	assert.GreaterOrEqual(t, result, 1)
}

func TestDetermineOptimalWorkers_UnknownMount(t *testing.T) {
	env := &Environment{
		MountType: MountTypeUnknown,
		IsRemote:  false,
	}

	sysRes := &SystemResources{
		CPUCores:       8,
		MemoryPressure: 0.5,
	}

	result := DetermineOptimalWorkers(env, sysRes)

	// Should use conservative estimate (CPU cores)
	assert.Equal(t, 8, result)
}

func TestEnvironment_Structure(t *testing.T) {
	env := &Environment{
		MountType:  MountTypeLocal,
		IsRemote:   false,
		FileSystem: "apfs",
		Path:       "/test/path",
	}

	assert.Equal(t, MountTypeLocal, env.MountType)
	assert.False(t, env.IsRemote)
	assert.Equal(t, "apfs", env.FileSystem)
	assert.Equal(t, "/test/path", env.Path)
}

func TestSystemResources_Structure(t *testing.T) {
	res := &SystemResources{
		CPUCores:       8,
		TotalMemoryMB:  16384,
		AvailMemoryMB:  8192,
		MemoryPressure: 0.5,
	}

	assert.Equal(t, 8, res.CPUCores)
	assert.Equal(t, uint64(16384), res.TotalMemoryMB)
	assert.Equal(t, uint64(8192), res.AvailMemoryMB)
	assert.Equal(t, 0.5, res.MemoryPressure)
}