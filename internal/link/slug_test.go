package link

import (
	"math/rand"
	"sort"
	"strings"
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

	s.T().Run("figure out the lowest value for 3 character slug", func(t *testing.T) {
		n := sort.Search(1<<30, func(i int) bool {
			return len(EncodeSlug(int64(i))) >= 3
		})
		require.Equal(t, 4096, n)
		require.Equal(t, 3, len(EncodeSlug(int64(n))))
		require.Equal(t, 2, len(EncodeSlug(int64(n-1))))
	})
}

func (s *SlugTestSuite) TestValidateSlug() {
	s.T().Run("accepts valid slug with alphanumeric", func(t *testing.T) {
		err := ValidateSlug("mylink123")
		require.NoError(t, err)
	})

	s.T().Run("accepts slug with uppercase letters", func(t *testing.T) {
		err := ValidateSlug("MyLink")
		require.NoError(t, err)
	})

	s.T().Run("accepts slug with hyphens", func(t *testing.T) {
		err := ValidateSlug("my-link")
		require.NoError(t, err)
	})

	s.T().Run("accepts slug with underscores", func(t *testing.T) {
		err := ValidateSlug("my_link")
		require.NoError(t, err)
	})

	s.T().Run("rejects empty slug", func(t *testing.T) {
		err := ValidateSlug("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "required")
	})

	s.T().Run("rejects slug with 1 character", func(t *testing.T) {
		err := ValidateSlug("a")
		require.Error(t, err)
		require.Contains(t, err.Error(), "at least 3")
	})

	s.T().Run("rejects slug with 2 characters", func(t *testing.T) {
		err := ValidateSlug("ab")
		require.Error(t, err)
		require.Contains(t, err.Error(), "at least 3")
	})

	s.T().Run("accepts slug with 3 characters", func(t *testing.T) {
		err := ValidateSlug("abc")
		require.NoError(t, err)
	})

	s.T().Run("rejects slug with invalid characters", func(t *testing.T) {
		err := ValidateSlug("invalid slug!")
		require.Error(t, err)
	})

	s.T().Run("rejects slug with dots", func(t *testing.T) {
		err := ValidateSlug("my.slug")
		require.Error(t, err)
	})

	s.T().Run("rejects slug with spaces", func(t *testing.T) {
		err := ValidateSlug("my slug")
		require.Error(t, err)
	})

	s.T().Run("rejects slug with slash", func(t *testing.T) {
		err := ValidateSlug("my/slug")
		require.Error(t, err)
	})

	s.T().Run("rejects slug too long", func(t *testing.T) {
		longSlug := strings.Repeat("a", 51)
		err := ValidateSlug(longSlug)
		require.Error(t, err)
		require.Contains(t, err.Error(), "50 characters")
	})

	s.T().Run("accepts slug at max length", func(t *testing.T) {
		maxSlug := strings.Repeat("a", 50)
		err := ValidateSlug(maxSlug)
		require.NoError(t, err)
	})
}

func TestSlugTestSuite(t *testing.T) {
	suite.Run(t, new(SlugTestSuite))
}
