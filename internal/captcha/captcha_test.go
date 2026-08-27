package captcha

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	altcha "github.com/altcha-org/altcha-lib-go/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CaptchaTestSuite struct {
	suite.Suite
	secret string
	cost   int
}

func (s *CaptchaTestSuite) SetupTest() {
	s.secret = "test-hmac-secret"
	s.cost = 100
}

func (s *CaptchaTestSuite) TestNew() {
	s.Run("returns disabled verifier when secret is empty", func() {
		v := New("", s.cost, 10*time.Minute)
		require.False(s.T(), v.Enabled())
	})

	s.Run("returns enabled verifier when secret is set", func() {
		v := New(s.secret, s.cost, 10*time.Minute)
		require.True(s.T(), v.Enabled())
	})
}

func (s *CaptchaTestSuite) TestNewChallenge() {
	s.Run("returns valid JSON challenge when enabled", func() {
		v := New(s.secret, s.cost, 10*time.Minute)
		challengeJSON, err := v.NewChallenge()
		require.NoError(s.T(), err)
		require.NotEmpty(s.T(), challengeJSON)

		var challenge altcha.Challenge
		err = json.Unmarshal(challengeJSON, &challenge)
		require.NoError(s.T(), err)
		require.NotEmpty(s.T(), challenge.Signature)
		require.NotEmpty(s.T(), challenge.Parameters.Algorithm)
		require.NotEmpty(s.T(), challenge.Parameters.Nonce)
		require.NotEmpty(s.T(), challenge.Parameters.Salt)
		require.Greater(s.T(), challenge.Parameters.Cost, 0)
		require.NotEmpty(s.T(), challenge.Parameters.KeyPrefix)
	})

	s.Run("returns error when disabled", func() {
		v := New("", s.cost, 10*time.Minute)
		_, err := v.NewChallenge()
		require.Error(s.T(), err)
		require.ErrorIs(s.T(), err, ErrDisabled)
	})

	s.Run("sets expiry timestamp", func() {
		expiry := 5 * time.Minute
		v := New(s.secret, s.cost, expiry)
		challengeJSON, err := v.NewChallenge()
		require.NoError(s.T(), err)

		var challenge altcha.Challenge
		err = json.Unmarshal(challengeJSON, &challenge)
		require.NoError(s.T(), err)
		require.Greater(s.T(), challenge.Parameters.ExpiresAt, time.Now().Unix())
	})
}

func (s *CaptchaTestSuite) TestVerify() {
	s.Run("verifies valid payload", func() {
		v := New(s.secret, s.cost, 10*time.Minute)

		challengeJSON, err := v.NewChallenge()
		require.NoError(s.T(), err)

		var challenge altcha.Challenge
		err = json.Unmarshal(challengeJSON, &challenge)
		require.NoError(s.T(), err)

		solution, err := altcha.SolveChallenge(altcha.SolveChallengeOptions{
			Challenge: challenge,
			DeriveKey: altcha.DeriveKeyPBKDF2(),
		})
		require.NoError(s.T(), err)
		require.NotNil(s.T(), solution)

		payload := altcha.Payload{
			Challenge: challenge,
			Solution:  *solution,
		}
		payloadJSON, err := json.Marshal(payload)
		require.NoError(s.T(), err)
		payloadB64 := base64.StdEncoding.EncodeToString(payloadJSON)

		err = v.Verify(payloadB64)
		require.NoError(s.T(), err)
	})

	s.Run("returns error when disabled", func() {
		v := New("", s.cost, 10*time.Minute)
		err := v.Verify("anything")
		require.Error(s.T(), err)
		require.ErrorIs(s.T(), err, ErrDisabled)
	})

	s.Run("returns error for empty payload", func() {
		v := New(s.secret, s.cost, 10*time.Minute)
		err := v.Verify("")
		require.Error(s.T(), err)
		require.ErrorIs(s.T(), err, ErrInvalidPayload)
	})

	s.Run("returns error for invalid base64", func() {
		v := New(s.secret, s.cost, 10*time.Minute)
		err := v.Verify("not-valid-base64!!!")
		require.Error(s.T(), err)
		require.ErrorIs(s.T(), err, ErrInvalidPayload)
	})

	s.Run("returns error for invalid JSON", func() {
		v := New(s.secret, s.cost, 10*time.Minute)
		payloadB64 := base64.StdEncoding.EncodeToString([]byte("not json"))
		err := v.Verify(payloadB64)
		require.Error(s.T(), err)
		require.ErrorIs(s.T(), err, ErrInvalidPayload)
	})

	s.Run("returns error for tampered signature", func() {
		v := New(s.secret, s.cost, 10*time.Minute)

		challengeJSON, err := v.NewChallenge()
		require.NoError(s.T(), err)

		var challenge altcha.Challenge
		err = json.Unmarshal(challengeJSON, &challenge)
		require.NoError(s.T(), err)

		solution, err := altcha.SolveChallenge(altcha.SolveChallengeOptions{
			Challenge: challenge,
			DeriveKey: altcha.DeriveKeyPBKDF2(),
		})
		require.NoError(s.T(), err)

		challenge.Signature = "tampered"
		payload := altcha.Payload{
			Challenge: challenge,
			Solution:  *solution,
		}
		payloadJSON, err := json.Marshal(payload)
		require.NoError(s.T(), err)
		payloadB64 := base64.StdEncoding.EncodeToString(payloadJSON)

		err = v.Verify(payloadB64)
		require.Error(s.T(), err)
		require.ErrorIs(s.T(), err, ErrVerificationFailed)
	})

	s.Run("returns error for wrong secret", func() {
		v1 := New("secret1", s.cost, 10*time.Minute)
		v2 := New("secret2", s.cost, 10*time.Minute)

		challengeJSON, err := v1.NewChallenge()
		require.NoError(s.T(), err)

		var challenge altcha.Challenge
		err = json.Unmarshal(challengeJSON, &challenge)
		require.NoError(s.T(), err)

		solution, err := altcha.SolveChallenge(altcha.SolveChallengeOptions{
			Challenge: challenge,
			DeriveKey: altcha.DeriveKeyPBKDF2(),
		})
		require.NoError(s.T(), err)

		payload := altcha.Payload{
			Challenge: challenge,
			Solution:  *solution,
		}
		payloadJSON, err := json.Marshal(payload)
		require.NoError(s.T(), err)
		payloadB64 := base64.StdEncoding.EncodeToString(payloadJSON)

		err = v2.Verify(payloadB64)
		require.Error(s.T(), err)
		require.ErrorIs(s.T(), err, ErrVerificationFailed)
	})

	s.Run("returns error for expired challenge", func() {
		v := New(s.secret, s.cost, 1*time.Second)

		challengeJSON, err := v.NewChallenge()
		require.NoError(s.T(), err)

		var challenge altcha.Challenge
		err = json.Unmarshal(challengeJSON, &challenge)
		require.NoError(s.T(), err)

		solution, err := altcha.SolveChallenge(altcha.SolveChallengeOptions{
			Challenge: challenge,
			DeriveKey: altcha.DeriveKeyPBKDF2(),
		})
		require.NoError(s.T(), err)

		time.Sleep(2 * time.Second)

		payload := altcha.Payload{
			Challenge: challenge,
			Solution:  *solution,
		}
		payloadJSON, err := json.Marshal(payload)
		require.NoError(s.T(), err)
		payloadB64 := base64.StdEncoding.EncodeToString(payloadJSON)

		err = v.Verify(payloadB64)
		require.Error(s.T(), err)
		require.ErrorIs(s.T(), err, ErrExpired)
	})
}

func TestCaptchaTestSuite(t *testing.T) {
	suite.Run(t, new(CaptchaTestSuite))
}
