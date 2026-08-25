package link

const base62Chars = "9hZPFa2KlLX6rTwJkHItzxyYs0Vd4bfAMUQGmvpCjRuoDieO5c7nNqWE38gS1B"

func EncodeBase62(n int64) string {
	if n == 0 {
		return string(base62Chars[0])
	}

	var result []byte
	for n > 0 {
		result = append([]byte{base62Chars[n%62]}, result...)
		n /= 62
	}
	return string(result)
}

func DecodeBase62(s string) int64 {
	var result int64
	for _, c := range s {
		result = result*62 + int64(charIndex(byte(c)))
	}
	return result
}

func charIndex(c byte) int {
	for i, ch := range base62Chars {
		if byte(ch) == c {
			return i
		}
	}
	return 0
}
