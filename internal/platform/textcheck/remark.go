// Package textcheck contains small deterministic checks for user-entered text.
package textcheck

import "strings"

// MeaningfulRemark accepts short, meaningful labels while rejecting empty and
// obvious keyboard-noise input. Semantic quality is handled separately by the
// optional AI classifier.
func MeaningfulRemark(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 300 {
		return false
	}
	lower := strings.ToLower(value)
	for _, noise := range []string{"asdf", "qwer", "zxcv", "hjkl", "1234"} {
		if strings.Contains(lower, noise) {
			return false
		}
	}
	letters := 0
	var previous rune
	repeated := 0
	for _, character := range lower {
		if character >= 'a' && character <= 'z' {
			letters++
			if character == previous {
				repeated++
			} else {
				repeated = 1
			}
			if repeated >= 4 {
				return false
			}
		}
		previous = character
	}
	return letters >= 2
}
