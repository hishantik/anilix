// Package match normalizes anime titles for exact-after-normalization
// matching. Shared by the AniList and Kitsu resolvers, which must agree
// on what "the same title" means.
package match

import (
	"strings"
	"unicode"
)

// romanNumeralDigits maps the multi-letter roman numerals that appear in
// titles onto decimal digits. A lone "i" is left alone: it is far more
// often the English pronoun than the number one.
var romanNumeralDigits = map[string]string{
	"ii": "2", "iii": "3", "iv": "4", "v": "5",
	"vi": "6", "vii": "7", "viii": "8", "ix": "9",
}

// Normalize lowercases a title and keeps only letters and digits, so
// "ONE PIECE" and "One Piece!" compare equal. Unicode-aware on purpose:
// AniList uses typographic punctuation ("Journey's End" with U+2019) while
// other sources use plain ASCII, and both must match.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Tokens splits a title into lowercase word tokens. Apostrophes join
// their neighbours ("Sibling's" is one word) while other punctuation and
// spaces separate words. Standalone roman numerals become digits so
// "Infinity Castle II" and "Infinity Castle Part 2" compare equal.
func Tokens(s string) []string {
	var tokens []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		tok := b.String()
		b.Reset()
		if d, ok := romanNumeralDigits[tok]; ok {
			tok = d
		}
		tokens = append(tokens, tok)
	}
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '\'' || r == '\u2019' || r == '`':
			// Join the word parts instead of splitting the word.
		default:
			flush()
		}
	}
	flush()
	return tokens
}

// TokensContainAll reports whether have contains every token of want,
// counting duplicates: two "movie" tokens in want need two in have.
func TokensContainAll(have, want []string) bool {
	counts := make(map[string]int, len(have))
	for _, t := range have {
		counts[t]++
	}
	for _, t := range want {
		if counts[t] == 0 {
			return false
		}
		counts[t]--
	}
	return true
}
