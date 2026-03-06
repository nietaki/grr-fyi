package signing

import (
	"crypto/sha256"
	"fmt"
	"time"
)

const SIGNATURE_VALIDITY_SECONDS = 300

var secret = "bnh!gng7waf3BKD-zgd"

func Init(newSecret string) {
	if newSecret != "" {
		secret = newSecret
	}
}

func EpochTime() int64 {
	return time.Now().Unix()
}

func RepTs(unixTime int64) string {
	return fmt.Sprintf("%d", unixTime)
}

func ParseTs(tsString string) int64 {
	var ts int64
	fmt.Sscanf(tsString, "%d", &ts)
	return ts
}

func Signature(filename string, unixTime int64) string {
	h := sha256.New()
	message := fmt.Sprintf("%s:%d:%s", filename, unixTime, secret)
	h.Write([]byte(message))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func SigningQueryString(filename string) string {
	ts := EpochTime()
	sig := Signature(filename, ts)
	return fmt.Sprintf("ts=%s&sig=%s", RepTs(ts), sig)
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func VerifySignature(filename string, ts int64, sig string) error {
	if abs(EpochTime()-ts) > SIGNATURE_VALIDITY_SECONDS {
		return fmt.Errorf("signature expired")
	}

	expectedSig := Signature(filename, ts)

	if sig != expectedSig {
		return fmt.Errorf("invalid signature")
	}
	return nil
}
