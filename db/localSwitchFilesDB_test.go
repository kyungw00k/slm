package db

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestAtomicProgressUpdate verifies that milestone-based progress updates
// prevent duplicate displays when multiple goroutines increment the counter
func TestAtomicProgressUpdate(t *testing.T) {
	var totalFilesFound int64
	var lastDisplayedMilestone int64
	var displayedMilestones sync.Map // Use sync.Map to track milestones thread-safely

	// Simulate concurrent goroutines finding files
	const numGoroutines = 10
	const filesPerGoroutine = 15
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < filesPerGoroutine; j++ {
				// Simulate finding a file - increment counter atomically
				currentTotal := atomic.AddInt64(&totalFilesFound, 1)

				// Calculate milestone
				var milestone int64
				if currentTotal == 1 {
					milestone = 1
				} else if currentTotal%10 == 0 {
					milestone = currentTotal
				}

				// Try to claim milestone using CAS
				if milestone > 0 {
					lastDisplayed := atomic.LoadInt64(&lastDisplayedMilestone)
					if milestone > lastDisplayed {
						if atomic.CompareAndSwapInt64(&lastDisplayedMilestone, lastDisplayed, milestone) {
							// Successfully claimed - record it
							displayedMilestones.Store(milestone, true)
						}
					}
				}
			}
		}()
	}

	wg.Wait()

	// Verify results
	expectedTotal := int64(numGoroutines * filesPerGoroutine)
	if totalFilesFound != expectedTotal {
		t.Errorf("Expected %d total files, got %d", expectedTotal, totalFilesFound)
	}

	// Collect and verify milestones
	milestones := make([]int64, 0)
	displayedMilestones.Range(func(key, value interface{}) bool {
		milestones = append(milestones, key.(int64))
		return true
	})

	// Verify expected milestones (1, 10, 20, ..., 140, 150)
	expectedMilestones := map[int64]bool{
		1: true, 10: true, 20: true, 30: true, 40: true, 50: true,
		60: true, 70: true, 80: true, 90: true, 100: true, 110: true,
		120: true, 130: true, 140: true, 150: true,
	}

	if len(milestones) != len(expectedMilestones) {
		t.Errorf("Expected %d unique milestones, got %d: %v", len(expectedMilestones), len(milestones), milestones)
	}

	// Verify each milestone is expected and there are no duplicates
	for _, milestone := range milestones {
		if !expectedMilestones[milestone] {
			t.Errorf("Unexpected milestone: %d", milestone)
		}
	}

	// Verify all expected milestones were found
	for expected := range expectedMilestones {
		found := false
		for _, m := range milestones {
			if m == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing expected milestone: %d", expected)
		}
	}

	t.Logf("Successfully processed %d files with %d unique milestones", totalFilesFound, len(milestones))
}

// TestAtomicProgressUpdateEdgeCases tests edge cases like first file and exact multiples
func TestAtomicProgressUpdateEdgeCases(t *testing.T) {
	testCases := []struct {
		name              string
		numFiles          int
		expectedMilestones []int64
	}{
		{
			name:              "Single file",
			numFiles:          1,
			expectedMilestones: []int64{1},
		},
		{
			name:              "Nine files",
			numFiles:          9,
			expectedMilestones: []int64{1},
		},
		{
			name:              "Exactly 10 files",
			numFiles:          10,
			expectedMilestones: []int64{1, 10},
		},
		{
			name:              "Exactly 100 files",
			numFiles:          100,
			expectedMilestones: []int64{1, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		},
		{
			name:              "105 files",
			numFiles:          105,
			expectedMilestones: []int64{1, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var totalFilesFound int64
			var lastDisplayedMilestone int64
			var displayedMilestones []int64

			// Sequential processing for predictable testing
			for i := 0; i < tc.numFiles; i++ {
				currentTotal := atomic.AddInt64(&totalFilesFound, 1)

				var milestone int64
				if currentTotal == 1 {
					milestone = 1
				} else if currentTotal%10 == 0 {
					milestone = currentTotal
				}

				if milestone > 0 {
					lastDisplayed := atomic.LoadInt64(&lastDisplayedMilestone)
					if milestone > lastDisplayed {
						if atomic.CompareAndSwapInt64(&lastDisplayedMilestone, lastDisplayed, milestone) {
							displayedMilestones = append(displayedMilestones, milestone)
						}
					}
				}
			}

			// Verify milestones match expected
			if len(displayedMilestones) != len(tc.expectedMilestones) {
				t.Errorf("Expected %d milestones, got %d: %v", len(tc.expectedMilestones), len(displayedMilestones), displayedMilestones)
				return
			}

			for i, expected := range tc.expectedMilestones {
				if displayedMilestones[i] != expected {
					t.Errorf("Milestone %d: expected %d, got %d", i, expected, displayedMilestones[i])
				}
			}
		})
	}
}
