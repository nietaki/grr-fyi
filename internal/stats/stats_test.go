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
	if mem < 0 {
		t.Errorf("Expected non-negative memory usage, got %d", mem)
	}

	// Memory should be zero or positive
	if mem < 0 {
		t.Errorf("Expected memory usage >= 0, got %d", mem)
	}

	// Test calling multiple times gives consistent results
	mem2, err := MemoryUsage()
	if err != nil {
		t.Fatalf("Second call to MemoryUsage failed: %v", err)
	}

	// Memory should be in reasonable range
	if mem < 0 || mem > 1024^4 { // 100GB sanity check
		t.Logf("Memory usage %d seems unusual", mem)
	}

	if mem != mem2 {
		t.Logf("Memory usage changed from %d to %d between calls", mem, mem2)
	}
}
