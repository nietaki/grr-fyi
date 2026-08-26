package link

const slugChars = "9hZPFa2KlLX6rTwJkHItzxyYs0Vd4bfAMUQGmvpCjRuoDieO5c7nNqWE38gS1B-_"

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
