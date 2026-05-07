package grading

import (
	"strings"
	"unicode"
)

const (
	StrictnessFlexible = "flexible"
	StrictnessStrict   = "strict"
)

// CheckAnswer reports whether userAnswer matches correctAnswer under the given strictness level.
func CheckAnswer(userAnswer, correctAnswer, strictness string) bool {
	if strictness == StrictnessStrict {
		return checkStrict(userAnswer, correctAnswer)
	}
	return checkFlexible(userAnswer, correctAnswer)
}

// checkStrict: case-insensitive exact match, all other characters must match.
func checkStrict(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// checkFlexible: case-insensitive, ignores punctuation, normalizes whitespace,
// and allows up to N character edits based on the Levenshtein distance.
func checkFlexible(a, b string) bool {
	na := normalize(a)
	nb := normalize(b)
	if na == nb {
		return true
	}
	threshold := levenshteinThreshold(len(nb))
	return levenshtein(na, nb) <= threshold
}

// normalize lowercases, removes punctuation, and collapses whitespace.
func normalize(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
			}
			prevSpace = true
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
		}
		// punctuation is dropped
	}
	return strings.TrimSpace(b.String())
}

// levenshteinThreshold returns the maximum allowed edit distance for a string of length n.
// len <= 4  → 0 (must match exactly after normalization)
// len 5–8   → 1
// len >= 9  → 2
func levenshteinThreshold(n int) int {
	switch {
	case n >= 9:
		return 2
	case n >= 5:
		return 1
	default:
		return 0
	}
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
