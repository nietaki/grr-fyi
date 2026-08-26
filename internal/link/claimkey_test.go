package link

import (
	"crypto/sha256"
	"encoding/hex"
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

func (s *ClaimKeyTestSuite) TestHashClaimKey() {
	s.T().Run("produces consistent hashes", func(t *testing.T) {
		key := "test-claim-key"
		hash1 := hashClaimKey(key)
		hash2 := hashClaimKey(key)
		require.Equal(t, hash1, hash2)
	})

	s.T().Run("produces different hashes for different keys", func(t *testing.T) {
		hash1 := hashClaimKey("key1")
		hash2 := hashClaimKey("key2")
		require.NotEqual(t, hash1, hash2)
	})

	s.T().Run("produces valid sha256 hex string", func(t *testing.T) {
		key := "test-key"
		hash := hashClaimKey(key)
		require.Len(t, hash, 64, "sha256 hex should be 64 characters")

		expected := sha256.Sum256([]byte(key))
		require.Equal(t, hex.EncodeToString(expected[:]), hash)
	})
}

func (s *ClaimKeyTestSuite) TestVerifyClaimKey() {
	s.T().Run("verifies correct claim key", func(t *testing.T) {
		key := "my-secret-key"
		l := &Link{ClaimKeyHash: hashClaimKey(key)}
		err := l.VerifyClaimKey(key)
		require.NoError(t, err)
	})

	s.T().Run("rejects incorrect claim key", func(t *testing.T) {
		l := &Link{ClaimKeyHash: hashClaimKey("correct-key")}
		err := l.VerifyClaimKey("wrong-key")
		require.ErrorIs(t, err, ErrInvalidClaim)
	})
}

func TestClaimKeyTestSuite(t *testing.T) {
	suite.Run(t, new(ClaimKeyTestSuite))
}
