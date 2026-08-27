package captcha

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"time"

	altcha "github.com/altcha-org/altcha-lib-go/v2"
)

var (
	ErrDisabled           = errors.New("captcha is disabled")
	ErrInvalidPayload     = errors.New("invalid captcha payload")
	ErrVerificationFailed = errors.New("captcha verification failed")
	ErrExpired            = errors.New("captcha challenge expired")
)

type Verifier interface {
	Enabled() bool
	NewChallenge() ([]byte, error)
	Verify(payloadB64 string) error
}

type verifier struct {
	secret    string
	cost      int
	expiry    time.Duration
	deriveKey altcha.DeriveKeyFunc
}

func New(secret string, cost int, expiry time.Duration) Verifier {
	if secret == "" {
		return &disabledVerifier{}
	}
	return &verifier{
		secret:    secret,
		cost:      cost,
		expiry:    expiry,
		deriveKey: altcha.DeriveKeyPBKDF2(),
	}
}

func (v *verifier) Enabled() bool {
	return true
}

func (v *verifier) NewChallenge() ([]byte, error) {
	counter := 5000 + rand.IntN(5000)
	expiresAt := time.Now().Add(v.expiry)
	challenge, err := altcha.CreateChallenge(altcha.CreateChallengeOptions{
		Algorithm:           "PBKDF2/SHA-256",
		DeriveKey:           v.deriveKey,
		HMACSignatureSecret: v.secret,
		Cost:                v.cost,
		KeyLength:           32,
		Counter:             &counter,
		ExpiresAt:           &expiresAt,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(challenge)
}

func (v *verifier) Verify(payloadB64 string) error {
	if payloadB64 == "" {
		return ErrInvalidPayload
	}
	decoded, err := base64.StdEncoding.DecodeString(payloadB64)
	if err != nil {
		return ErrInvalidPayload
	}
	var payload altcha.Payload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return ErrInvalidPayload
	}
	result, err := altcha.VerifySolution(altcha.VerifySolutionOptions{
		Challenge:           payload.Challenge,
		Solution:            payload.Solution,
		DeriveKey:           v.deriveKey,
		HMACSignatureSecret: v.secret,
	})
	if err != nil {
		return ErrVerificationFailed
	}
	if result.Expired {
		return ErrExpired
	}
	if !result.Verified {
		return ErrVerificationFailed
	}
	return nil
}

type disabledVerifier struct{}

func (d *disabledVerifier) Enabled() bool { return false }

func (d *disabledVerifier) NewChallenge() ([]byte, error) {
	return nil, ErrDisabled
}

func (d *disabledVerifier) Verify(payloadB64 string) error {
	return ErrDisabled
}
