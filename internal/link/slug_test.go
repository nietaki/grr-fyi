package link

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SlugTestSuite struct {
	suite.Suite
}

func (s *SlugTestSuite) TestEncodeSlug() {
	s.T().Run("operates on a 64 char, unique alphabet", func(t *testing.T) {
		require.Equal(t, 64, len(slugChars))

		unique := map[byte]struct{}{}
		for _, c := range slugChars {
			unique[byte(c)] = struct{}{}
		}
		require.Equal(t, 64, len(unique))
	})
}

func (s *SlugTestSuite) TestEncodeDecodeRoundtrip() {
	s.T().Run("doesn't have any obvious collisions", func(t *testing.T) {
		for range 100 {
			a := int64(rand.Intn(1001))
			encodedA := EncodeSlug(a)
			decodedA := DecodeSlug(encodedA)
			require.Equal(t, a, decodedA)
			b := int64(rand.Intn(1001))
			encodedB := EncodeSlug(b)
			decodedB := DecodeSlug(encodedB)
			require.Equal(t, b, decodedB)
			require.Equal(t, a == b, encodedA == encodedB)
		}
	})
	s.T().Run("roundtrips various values", func(t *testing.T) {
		values := []int64{0, 1, 25, 26, 51, 52, 61, 62, 63, 64, 100, 1000, 1575, 3843, 3844, 100000, 1000000}
		for _, v := range values {
			encoded := EncodeSlug(v)
			decoded := DecodeSlug(encoded)
			require.Equal(t, v, decoded)
		}
	})

	s.T().Run("figure out the lowest value for 4 character slug", func(t *testing.T) {
		n := sort.Search(1<<30, func(i int) bool {
			return len(EncodeSlug(int64(i))) >= 4
		})
		require.Equal(t, 262144, n)
		require.Equal(t, 4, len(EncodeSlug(int64(n))))
		require.Equal(t, 3, len(EncodeSlug(int64(n-1))))
	})
}

func TestSlugTestSuite(t *testing.T) {
	suite.Run(t, new(SlugTestSuite))
}
