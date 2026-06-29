package tokenstats

import "math"

// EstimateTokens 字符加权法估算 token 数
// asciiChars / charsPerToken + nonAsciiChars / 1.5，向上取整
func EstimateTokens(s string, charsPerToken float64) int {
	if s == "" {
		return 0
	}
	var ascii, nonASCII int
	for _, r := range s {
		if r < 128 {
			ascii++
		} else {
			nonASCII++
		}
	}
	tokens := float64(ascii)/charsPerToken + float64(nonASCII)/1.5
	return int(math.Ceil(tokens))
}
