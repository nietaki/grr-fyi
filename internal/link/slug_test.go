package link

import (
	"github.com/mvrahden/go-test/pkg/gotest"
)

type SlugTestSuite struct{}

func (s *SlugTestSuite) BeforeEach(t *gotest.T) {}

func (s *SlugTestSuite) TestEncodeBase62(t *gotest.T) {
	t.It("encodes 0 as 'a'", func(it *gotest.T) {
		gotest.Equal(it, "a", encodeBase62(0))
	})

	t.It("encodes 1 as 'b'", func(it *gotest.T) {
		gotest.Equal(it, "b", encodeBase62(1))
	})

	t.It("encodes 25 as 'z'", func(it *gotest.T) {
		gotest.Equal(it, "z", encodeBase62(25))
	})

	t.It("encodes 26 as 'A'", func(it *gotest.T) {
		gotest.Equal(it, "A", encodeBase62(26))
	})

	t.It("encodes 51 as 'Z'", func(it *gotest.T) {
		gotest.Equal(it, "Z", encodeBase62(51))
	})

	t.It("encodes 52 as '0'", func(it *gotest.T) {
		gotest.Equal(it, "0", encodeBase62(52))
	})

	t.It("encodes 61 as '9'", func(it *gotest.T) {
		gotest.Equal(it, "9", encodeBase62(61))
	})

	t.It("encodes 62 as 'ba'", func(it *gotest.T) {
		gotest.Equal(it, "ba", encodeBase62(62))
	})

	t.It("encodes 63 as 'bb'", func(it *gotest.T) {
		gotest.Equal(it, "bb", encodeBase62(63))
	})

	t.It("encodes 3843 as '99'", func(it *gotest.T) {
		gotest.Equal(it, "99", encodeBase62(3843))
	})

	t.It("encodes 3844 as 'baa'", func(it *gotest.T) {
		gotest.Equal(it, "baa", encodeBase62(3844))
	})
}

func (s *SlugTestSuite) TestDecodeBase62(t *gotest.T) {
	t.It("decodes 'a' as 0", func(it *gotest.T) {
		gotest.Equal(it, int64(0), decodeBase62("a"))
	})

	t.It("decodes 'z' as 25", func(it *gotest.T) {
		gotest.Equal(it, int64(25), decodeBase62("z"))
	})

	t.It("decodes 'A' as 26", func(it *gotest.T) {
		gotest.Equal(it, int64(26), decodeBase62("A"))
	})

	t.It("decodes '9' as 61", func(it *gotest.T) {
		gotest.Equal(it, int64(61), decodeBase62("9"))
	})

	t.It("decodes 'ba' as 62", func(it *gotest.T) {
		gotest.Equal(it, int64(62), decodeBase62("ba"))
	})

	t.It("decodes '99' as 3843", func(it *gotest.T) {
		gotest.Equal(it, int64(3843), decodeBase62("99"))
	})

	t.It("decodes 'baa' as 3844", func(it *gotest.T) {
		gotest.Equal(it, int64(3844), decodeBase62("baa"))
	})
}

func (s *SlugTestSuite) TestEncodeDecodeRoundtrip(t *gotest.T) {
	t.It("roundtrips various values", func(it *gotest.T) {
		values := []int64{0, 1, 25, 26, 51, 52, 61, 62, 63, 100, 1000, 1575, 3843, 3844, 100000, 1000000}
		for _, v := range values {
			encoded := encodeBase62(v)
			decoded := decodeBase62(encoded)
			gotest.Equal(it, v, decoded)
		}
	})
}
