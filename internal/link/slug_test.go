package link

import (
	"math/rand"

	"github.com/mvrahden/go-test/pkg/gotest"
)

type SlugTestSuite struct{}

func (s *SlugTestSuite) BeforeEach(t *gotest.T) {}

func (s *SlugTestSuite) TestEncodeBase62(t *gotest.T) {
	t.It("operates on a 62 char, unique alphabet", func(it *gotest.T) {
		gotest.Equal(it, 62, len(base62Chars))

		unique := map[byte]struct{}{}
		for _, c := range base62Chars {
			unique[byte(c)] = struct{}{}
		}
		gotest.Equal(it, 62, len(unique))
	})
}

func (s *SlugTestSuite) TestEncodeDecodeRoundtrip(t *gotest.T) {
	t.It("doesn't have any obvious collisions", func(it *gotest.T) {
		// get two random int64 values in the range of 0..1000 and assert that the encoded values are
		// only the same if the values are the same
		for i := 0; i < 100; i++ {
			a := int64(rand.Intn(1001))
			encodedA := EncodeBase62(a)
			decodedA := DecodeBase62(encodedA)
			gotest.Equal(it, a, decodedA)
			b := int64(rand.Intn(1001))
			encodedB := EncodeBase62(b)
			decodedB := DecodeBase62(encodedB)
			gotest.Equal(it, b, decodedB)
			gotest.Equal(it, a == b, encodedA == encodedB)
		}
	})
	t.It("roundtrips various values", func(it *gotest.T) {
		values := []int64{0, 1, 25, 26, 51, 52, 61, 62, 63, 100, 1000, 1575, 3843, 3844, 100000, 1000000}
		for _, v := range values {
			encoded := EncodeBase62(v)
			decoded := DecodeBase62(encoded)
			gotest.Equal(it, v, decoded)
		}
	})
}
