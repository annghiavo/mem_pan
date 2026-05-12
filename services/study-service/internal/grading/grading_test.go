package grading

import "testing"

// ---------------------------------------------------------------------------
// CheckAnswer – strict mode
// ---------------------------------------------------------------------------

func TestCheckAnswer_Strict_ExactMatch(t *testing.T) {
	if !CheckAnswer("hello", "hello", StrictnessStrict) {
		t.Error("expected exact match to pass")
	}
}

func TestCheckAnswer_Strict_CaseInsensitive(t *testing.T) {
	if !CheckAnswer("Hello", "hello", StrictnessStrict) {
		t.Error("expected case-insensitive match to pass in strict mode")
	}
}

func TestCheckAnswer_Strict_LeadingTrailingSpaces(t *testing.T) {
	if !CheckAnswer("  hello  ", "hello", StrictnessStrict) {
		t.Error("expected trimmed whitespace to pass in strict mode")
	}
}

func TestCheckAnswer_Strict_InternalSpaceMismatch(t *testing.T) {
	// Strict mode does not collapse internal spaces.
	if CheckAnswer("hel lo", "hello", StrictnessStrict) {
		t.Error("internal space mismatch should fail in strict mode")
	}
}

func TestCheckAnswer_Strict_PunctuationMismatch(t *testing.T) {
	if CheckAnswer("hello!", "hello", StrictnessStrict) {
		t.Error("punctuation difference should fail in strict mode")
	}
}

func TestCheckAnswer_Strict_WrongAnswer(t *testing.T) {
	if CheckAnswer("world", "hello", StrictnessStrict) {
		t.Error("wrong answer should fail")
	}
}

// ---------------------------------------------------------------------------
// CheckAnswer – flexible mode
// ---------------------------------------------------------------------------

func TestCheckAnswer_Flexible_ExactMatch(t *testing.T) {
	if !CheckAnswer("hello", "hello", StrictnessFlexible) {
		t.Error("expected exact match to pass in flexible mode")
	}
}

func TestCheckAnswer_Flexible_CaseInsensitive(t *testing.T) {
	if !CheckAnswer("HELLO", "hello", StrictnessFlexible) {
		t.Error("expected case-insensitive match in flexible mode")
	}
}

func TestCheckAnswer_Flexible_PunctuationIgnored(t *testing.T) {
	if !CheckAnswer("hello!", "hello", StrictnessFlexible) {
		t.Error("punctuation should be ignored in flexible mode")
	}
}

func TestCheckAnswer_Flexible_NormalizedWhitespace(t *testing.T) {
	if !CheckAnswer("hello   world", "hello world", StrictnessFlexible) {
		t.Error("collapsed whitespace should match in flexible mode")
	}
}

// Short word (≤4 chars): zero edits allowed after normalization.
func TestCheckAnswer_Flexible_ShortWord_ExactRequired(t *testing.T) {
	if CheckAnswer("ab", "ac", StrictnessFlexible) {
		t.Error("short word (≤4 chars) must match exactly after normalization")
	}
}

func TestCheckAnswer_Flexible_ShortWord_MatchesExactly(t *testing.T) {
	if !CheckAnswer("cat", "cat", StrictnessFlexible) {
		t.Error("short word exact match should pass")
	}
}

// Medium word (5–8 chars): one edit allowed.
func TestCheckAnswer_Flexible_MediumWord_OneEditAllowed(t *testing.T) {
	// "apple" → "applo": one substitution (threshold = 1 for len 5)
	if !CheckAnswer("applo", "apple", StrictnessFlexible) {
		t.Error("one-edit typo should pass for medium word")
	}
}

func TestCheckAnswer_Flexible_MediumWord_TwoEditsFail(t *testing.T) {
	// "apple" → "aplpo": two substitutions (should fail for len 5)
	if CheckAnswer("aplpo", "apple", StrictnessFlexible) {
		t.Error("two-edit typo should fail for medium word")
	}
}

// Long word (≥9 chars): two edits allowed.
func TestCheckAnswer_Flexible_LongWord_TwoEditsAllowed(t *testing.T) {
	// "algorithm" (9 chars) → "algerithm" (one substitution) → also try two substitutions
	if !CheckAnswer("algorithem", "algorithm", StrictnessFlexible) {
		t.Error("two-edit typo should pass for long word (len ≥9)")
	}
}

func TestCheckAnswer_Flexible_LongWord_ThreeEditsFail(t *testing.T) {
	// "xlgorxthx": positions 0 (a→x), 5 (i→x), 8 (m→x) — three substitutions.
	// levenshtein = 3, threshold for len 9 = 2, so it should fail.
	if CheckAnswer("xlgorxthx", "algorithm", StrictnessFlexible) {
		t.Error("three-edit typo should fail for long word")
	}
}

func TestCheckAnswer_Flexible_UnknownStrictnessFallsToFlexible(t *testing.T) {
	// An unrecognised strictness string falls through to flexible.
	if !CheckAnswer("hello!", "hello", "unknown") {
		t.Error("unrecognised strictness should fall back to flexible behaviour")
	}
}

// ---------------------------------------------------------------------------
// normalize
// ---------------------------------------------------------------------------

func TestNormalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Hello, World!", "hello world"},
		{"  leading  trailing  ", "leading trailing"},
		{"café", "café"},
		{"it's a test", "its a test"},
		{"UPPER CASE", "upper case"},
		{"", ""},
		{"   ", ""},
		{"123 abc", "123 abc"},
	}
	for _, tc := range cases {
		got := normalize(tc.in)
		if got != tc.want {
			t.Errorf("normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// levenshteinThreshold
// ---------------------------------------------------------------------------

func TestLevenshteinThreshold(t *testing.T) {
	cases := []struct {
		n    int
		want int
	}{
		{0, 0},
		{1, 0},
		{4, 0},
		{5, 1},
		{8, 1},
		{9, 2},
		{20, 2},
	}
	for _, tc := range cases {
		got := levenshteinThreshold(tc.n)
		if got != tc.want {
			t.Errorf("levenshteinThreshold(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// levenshtein
// ---------------------------------------------------------------------------

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "axc", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
	}
	for _, tc := range cases {
		got := levenshtein(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
