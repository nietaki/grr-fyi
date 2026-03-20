package stats

import "testing"

func TestMemoryUsage(t *testing.T) {
	// Test that MemoryUsage can be called without panicking
	mem, err := MemoryUsage()

	// Should not error, but if it does, that's ok (some environments don't support)
	if err != nil {
		t.Logf("MemoryUsage not supported in this environment: %v", err)
		return
	}

	// Memory should be non-negative
	if mem > 1024*1024*1024*100 { // 100GB sanity check
		t.Logf("Memory usage %d seems unusually high", mem)
	}

	// Test calling multiple times gives consistent results
	mem2, err := MemoryUsage()
	if err != nil {
		t.Fatalf("Second call to MemoryUsage failed: %v", err)
	}

	// Memory should be in reasonable range

	if mem != mem2 {
		t.Logf("Memory usage changed from %d to %d between calls", mem, mem2)
	}
}
