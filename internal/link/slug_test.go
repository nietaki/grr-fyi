package link

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SlugTestSuite struct {
	suite.Suite
}

func (s *SlugTestSuite) TestEncodeBase62() {
	s.T().Run("operates on a 62 char, unique alphabet", func(t *testing.T) {
		require.Equal(t, 62, len(base62Chars))

		unique := map[byte]struct{}{}
		for _, c := range base62Chars {
			unique[byte(c)] = struct{}{}
		}
		require.Equal(t, 62, len(unique))
	})
}

func (s *SlugTestSuite) TestEncodeDecodeRoundtrip() {
	s.T().Run("doesn't have any obvious collisions", func(t *testing.T) {
		for range 100 {
			a := int64(rand.Intn(1001))
			encodedA := EncodeBase62(a)
			decodedA := DecodeBase62(encodedA)
			require.Equal(t, a, decodedA)
			b := int64(rand.Intn(1001))
			encodedB := EncodeBase62(b)
			decodedB := DecodeBase62(encodedB)
			require.Equal(t, b, decodedB)
			require.Equal(t, a == b, encodedA == encodedB)
		}
	})
	s.T().Run("roundtrips various values", func(t *testing.T) {
		values := []int64{0, 1, 25, 26, 51, 52, 61, 62, 63, 100, 1000, 1575, 3843, 3844, 100000, 1000000}
		for _, v := range values {
			encoded := EncodeBase62(v)
			decoded := DecodeBase62(encoded)
			require.Equal(t, v, decoded)
		}
	})
}

func TestSlugTestSuite(t *testing.T) {
	suite.Run(t, new(SlugTestSuite))
}
