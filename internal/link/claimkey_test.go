package link

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ClaimKeyTestSuite struct {
	suite.Suite
}

func (s *ClaimKeyTestSuite) TestGenerateClaimKey() {
	s.T().Run("generates a non-empty claim key", func(t *testing.T) {
		key := generateClaimKey()
		require.NotEqual(t, "", key)
	})

	s.T().Run("generates different keys each time", func(t *testing.T) {
		key1 := generateClaimKey()
		key2 := generateClaimKey()

		require.NotEqual(t, key1, key2)
	})
}

func TestClaimKeyTestSuite(t *testing.T) {
	suite.Run(t, new(ClaimKeyTestSuite))
}
