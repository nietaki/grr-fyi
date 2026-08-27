package link

import (
	"errors"
	"regexp"
)

const slugChars = "9hZPFa2KlLX6rTwJkHItzxyYs0Vd4bfAMUQGmvpCjRuoDieO5c7nNqWE38gS1B-_"

var slugRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)

// ValidateSlug checks that the slug is valid for use as a URL path segment.
//
// Rules:
//   - Must not be empty
//   - Must be at least 3 characters
//   - Must be 50 characters or less
//   - Can only contain letters, numbers, dashes, and underscores
//
// Returns an error with a human-readable message if validation fails.
func ValidateSlug(slug string) error {
	if slug == "" {
		return errors.New("slug is required")
	}

	if len(slug) < 3 {
		return errors.New("slug must be at least 3 characters")
	}

	if len(slug) > 50 {
		return errors.New("slug must be 50 characters or less")
	}

	if !slugRegex.MatchString(slug) {
		return errors.New("slug can only contain letters, numbers, dashes, and underscores")
	}

	return nil
}

func EncodeSlug(n int64) string {
	if n == 0 {
		return string(slugChars[0])
	}

	var result []byte
	for n > 0 {
		result = append([]byte{slugChars[n%64]}, result...)
		n /= 64
	}
	return string(result)
}

func DecodeSlug(s string) int64 {
	var result int64
	for _, c := range s {
		result = result*64 + int64(charIndex(byte(c)))
	}
	return result
}

func charIndex(c byte) int {
	for i, ch := range slugChars {
		if byte(ch) == c {
			return i
		}
	}
	return 0
}
