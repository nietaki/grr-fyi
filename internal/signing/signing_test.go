package signing

import (
	"strings"
	"testing"
)

func TestSignature(t *testing.T) {
	// Test basic signature generation
	filename := "raw_data/test.pdf"
	timestamp := int64(1234567890)

	sig := Signature(filename, timestamp)

	// Verify signature is 64 hex characters
	if len(sig) != 64 {
		t.Errorf("Expected signature length 64, got %d", len(sig))
	}

	// Verify signature is deterministic
	sig2 := Signature(filename, timestamp)
	if sig != sig2 {
		t.Errorf("Expected deterministic signature, got %s and %s", sig, sig2)
	}

	// Different timestamps should produce different signatures
	sig3 := Signature(filename, timestamp+1)
	if sig == sig3 {
		t.Errorf("Expected different signatures for different timestamps")
	}

	// Different filenames should produce different signatures
	sig4 := Signature("raw_data/other.pdf", timestamp)
	if sig == sig4 {
		t.Errorf("Expected different signatures for different filenames")
	}
}

func TestSigningQueryString(t *testing.T) {
	filename := "raw_data/test.pdf"

	queryString := SigningQueryString(filename)

	// Verify query string contains ts and sig parameters
	if queryString == "" {
		t.Errorf("Expected non-empty query string")
	}

	// Verify it's in the format ts=123456&sig=abc123...
	if !strings.Contains(queryString, "ts=") {
		t.Errorf("Expected query string to contain 'ts=' parameter")
	}

	if !strings.Contains(queryString, "sig=") {
		t.Errorf("Expected query string to contain 'sig=' parameter")
	}
}

func TestParseTs(t *testing.T) {
	// Test valid timestamp parsing
	tsString := "1234567890"
	ts := ParseTs(tsString)

	if ts != int64(1234567890) {
		t.Errorf("Expected timestamp 1234567890, got %d", ts)
	}

	// Test edge case: 0
	zeroString := "0"
	zeroTs := ParseTs(zeroString)
	if zeroTs != int64(0) {
		t.Errorf("Expected timestamp 0, got %d", zeroTs)
	}

	// Test negative timestamp
	negativeString := "-12345"
	negativeTs := ParseTs(negativeString)
	if negativeTs != int64(-12345) {
		t.Errorf("Expected timestamp -12345, got %d", negativeTs)
	}
}

func TestVerifySignature(t *testing.T) {
	filename := "raw_data/test.pdf"
	originalTs := EpochTime()

	// Create valid signature
	sig := Signature(filename, originalTs)

	// Verify valid signature passes
	if err := VerifySignature(filename, originalTs, sig); err != nil {
		t.Errorf("Expected valid signature to verify: %v", err)
	}

	// Test with invalid signature
	invalidSig := Signature(filename, originalTs) + "1" // Tamper with signature
	if err := VerifySignature(filename, originalTs, invalidSig); err == nil {
		t.Errorf("Expected invalid signature to fail verification")
	}
}

func TestVerifySignatureExpiry(t *testing.T) {
	filename := "raw_data/test.pdf"

	// Test expired signature (timestamp too old)
	oldTs := EpochTime() - SIGNATURE_VALIDITY_SECONDS - 1
	sig := Signature(filename, oldTs)

	if err := VerifySignature(filename, oldTs, sig); err == nil {
		t.Errorf("Expected expired signature to fail verification")
	}

	// Test signature from far future (timestamp too new)
	futureTs := EpochTime() + SIGNATURE_VALIDITY_SECONDS + 1
	sig2 := Signature(filename, futureTs)

	if err := VerifySignature(filename, futureTs, sig2); err == nil {
		t.Errorf("Expected future signature to fail verification")
	}
}

func TestAbsForExpireWindow(t *testing.T) {
	// Verify abs function works correctly for timestamp window checking
	if abs(10) != 10 {
		t.Errorf("Expected abs(10)=10")
	}
	if abs(-10) != 10 {
		t.Errorf("Expected abs(-10)=10")
	}
	if abs(0) != 0 {
		t.Errorf("Expected abs(0)=0")
	}
}

func TestInit(t *testing.T) {
	// Test that Init changes the secret
	oldSecret := secret

	newSecret := "my-new-secret-key"
	Init(newSecret)

	if secret != newSecret {
		t.Errorf("Expected secret to be '%s', got '%s'", newSecret, secret)
	}

	// Reset to original
	Init(oldSecret)

	// Test that empty string doesn't change the secret
	Init("")
	if secret != oldSecret {
		t.Errorf("Expected secret to remain unchanged with empty Init()")
	}
}
