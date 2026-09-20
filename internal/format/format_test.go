package format

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type FormatTestSuite struct {
	suite.Suite
}

func TestFormatSuite(t *testing.T) {
	suite.Run(t, new(FormatTestSuite))
}

func (s *FormatTestSuite) TestBytes() {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "\u2014"},
		{-5, "\u2014"},
		{999, "999 B"},
		{1024, "1.0 kB"},
		{1536, "1.5 kB"},
		{1_000_000, "1.0 MB"},
		{82_854_982, "83 MB"},
		{1_073_741_824, "1.1 GB"},
	}
	for _, tc := range cases {
		s.T().Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, Bytes(tc.in))
		})
	}
}

func (s *FormatTestSuite) TestCount() {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{7, "7"},
		{999, "999"},
		{1234567, "1,234,567"},
		{-1400, "-1,400"},
	}
	for _, tc := range cases {
		s.T().Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, Count(tc.in))
		})
	}
}

func (s *FormatTestSuite) TestNumber() {
	s.Assert().Equal("\u2014", Number(math.NaN()))
	s.Assert().Equal("\u2014", Number(math.Inf(1)))
	s.Assert().Equal("\u2014", Number(math.Inf(-1)))
	s.Assert().Equal("156.4", Number(156.4))
	s.Assert().Equal("1,234.5", Number(1234.55)) // half-to-even on the float repr
	s.Assert().Equal("-18", Number(-18))
}

func (s *FormatTestSuite) TestSecondsToDuration() {
	cases := []struct {
		sec  float64
		want time.Duration
	}{
		{0, 0},
		{30, 30 * time.Second},
		{59.4, 59 * time.Second},
		{60.5, 61 * time.Second}, // .5 rounds away from zero
		{-30, -30 * time.Second},
	}
	for _, tc := range cases {
		s.T().Run(tc.want.String(), func(t *testing.T) {
			assert.Equal(t, tc.want, SecondsToDuration(tc.sec))
		})
	}
}

func (s *FormatTestSuite) TestUptime() {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{45 * time.Second, "45s"},
		{59*time.Second + 900*time.Millisecond, "59s"},
		{2 * time.Minute, "2m"},
		{61 * time.Minute, "1h 1m"},
		{25 * time.Hour, "1d 1h 0m"},
		{74*time.Hour + 30*time.Minute, "3d 2h 30m"},
	}
	for _, tc := range cases {
		s.T().Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, Uptime(tc.in))
		})
	}
}
