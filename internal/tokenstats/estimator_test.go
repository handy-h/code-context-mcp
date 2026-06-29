package tokenstats

import "testing"

func TestEstimateTokens_PureASCII(t *testing.T) {
	// "hello world" = 11 ASCII chars, 11/4.0 = 2.75 → ceil = 3
	got := EstimateTokens("hello world", 4.0)
	if got != 3 {
		t.Errorf("EstimateTokens(\"hello world\", 4.0) = %d, want 3", got)
	}
}

func TestEstimateTokens_Empty(t *testing.T) {
	got := EstimateTokens("", 4.0)
	if got != 0 {
		t.Errorf("EstimateTokens(\"\", 4.0) = %d, want 0", got)
	}
}

func TestEstimateTokens_ChineseMixed(t *testing.T) {
	// "hello世界" = 5 ASCII + 2 non-ASCII
	// 5/4.0 + 2/1.5 = 1.25 + 1.333 = 2.583 → ceil = 3
	got := EstimateTokens("hello世界", 4.0)
	if got != 3 {
		t.Errorf("EstimateTokens(\"hello世界\", 4.0) = %d, want 3", got)
	}
}

func TestEstimateTokens_CustomCharsPerToken(t *testing.T) {
	// "abcdefghij" = 10 ASCII, 10/3.5 = 2.857 → ceil = 3
	got := EstimateTokens("abcdefghij", 3.5)
	if got != 3 {
		t.Errorf("EstimateTokens(\"abcdefghij\", 3.5) = %d, want 3", got)
	}
}

func TestEstimateTokens_PureChinese(t *testing.T) {
	// "你好世界" = 4 non-ASCII, 4/1.5 = 2.667 → ceil = 3
	got := EstimateTokens("你好世界", 4.0)
	if got != 3 {
		t.Errorf("EstimateTokens(\"你好世界\", 4.0) = %d, want 3", got)
	}
}
