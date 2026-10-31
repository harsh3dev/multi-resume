package checks

import (
	"regexp"
	"strings"
)

// AI safety flags.
type Flag string

const (
	FlagNewClaim      Flag = "new_claim"
	FlagNumberChanged Flag = "number_changed"
	FlagTooLong       Flag = "too_long"
	FlagPageOverflow  Flag = "page_overflow"
)

var numberToken = regexp.MustCompile(`\d+(?:[.,:/-]\d+)*%?\+?`)

// CheckNumbers reports whether every number in newText also appears in orig.
// An AI may drop or keep numbers but must never invent one.
func CheckNumbers(orig, new string) bool {
	origNums := map[string]int{}
	for _, n := range numberToken.FindAllString(orig, -1) {
		origNums[n]++
	}
	for _, n := range numberToken.FindAllString(new, -1) {
		if origNums[n] == 0 {
			return false
		}
		origNums[n]--
	}
	return true
}

var techWord = regexp.MustCompile(`\b[A-Z][a-zA-Z0-9]*(?:\.[a-z]+)?(?:\+\+|#)?\b`)

// stopWords are capitalized English words that are not technology claims.
var stopWords = map[string]bool{
	"A": true, "I": true, "The": true, "This": true, "We": true, "My": true,
	"Jan": true, "Feb": true, "Mar": true, "Apr": true, "May": true, "Jun": true,
	"Jul": true, "Aug": true, "Sep": true, "Sept": true, "Oct": true, "Nov": true, "Dec": true,
	"January": true, "February": true, "March": true, "April": true, "June": true,
	"July": true, "August": true, "September": true, "October": true, "November": true, "December": true,
	"Led": true, "Built": true, "Developed": true, "Improved": true, "Reduced": true,
	"Designed": true, "Implemented": true, "Created": true, "Worked": true, "Using": true,
	"Used": true, "Wrote": true, "Maintained": true, "Contributed": true, "Presented": true,
	"Deployed": true, "Managed": true, "Explored": true, "Conducted": true, "Communicated": true,
	"Assessed": true, "Visualized": true, "Published": true, "Collaborated": true,
}

// CheckNewClaims returns capitalized candidate words in newText that appear
// nowhere in the resume corpus. Each is a potential invented skill.
func CheckNewClaims(corpus, new string) []string {
	lower := strings.ToLower(corpus)
	var out []string
	seen := map[string]bool{}
	for _, w := range techWord.FindAllString(new, -1) {
		if len(w) < 2 || stopWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		if !strings.Contains(lower, strings.ToLower(w)) {
			out = append(out, w)
		}
	}
	return out
}

// CheckMaxLen reports whether new fits the cap.
func CheckMaxLen(new string, maxLen int) bool {
	return len(new) <= maxLen
}
