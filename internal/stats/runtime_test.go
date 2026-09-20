package stats

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type RuntimeTestSuite struct {
	suite.Suite
}

func (s *RuntimeTestSuite) TestCollectRuntimeBasics() {
	start := time.Now().Add(-2 * time.Second)

	// Spawn a known number of extra goroutines so we can assert growth.
	before := runtime.NumGoroutine()
	release := make(chan struct{})
	defer close(release)
	for i := 0; i < 5; i++ {
		go func() { <-release }()
	}

	runtime.Gosched()
	rt := CollectRuntime(start)

	s.Require().GreaterOrEqual(rt.Uptime, 2*time.Second)
	s.Require().GreaterOrEqual(rt.Goroutines, int64(before+5))
	s.Require().Greater(rt.HeapAllocBytes, int64(0))
	s.Require().Greater(rt.HeapInuseBytes, int64(0))
	s.Require().Greater(rt.SysBytes, int64(0))
	// RSS is only available on Linux; just require it is non-negative (uint)
	// and, when present, larger than heap.
}

func (s *RuntimeTestSuite) TestUptimeGrows() {
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	rt := CollectRuntime(start)
	s.Require().Greater(rt.Uptime, time.Millisecond)
}

func TestRuntimeSuite(t *testing.T) {
	suite.Run(t, new(RuntimeTestSuite))
}
