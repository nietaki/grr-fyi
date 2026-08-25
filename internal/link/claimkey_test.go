package link

import (
	"github.com/mvrahden/go-test/pkg/gotest"
)

type ClaimKeyTestSuite struct{}

func (s *ClaimKeyTestSuite) BeforeEach(t *gotest.T) {}

func (s *ClaimKeyTestSuite) TestGenerateClaimKey(t *gotest.T) {
	t.It("generates a non-empty claim key", func(it *gotest.T) {
		key, err := generateClaimKey()
		gotest.NoError(it, err, "generateClaimKey")
		gotest.NotEqual(it, "", key)
	})

	t.It("generates different keys each time", func(it *gotest.T) {
		key1, err := generateClaimKey()
		gotest.NoError(it, err, "generateClaimKey 1")

		key2, err := generateClaimKey()
		gotest.NoError(it, err, "generateClaimKey 2")

		gotest.NotEqual(it, key1, key2)
	})
}
