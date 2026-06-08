// Package text holds generic string-matching utilities used by the CLI for
// "did you mean…?" suggestions. It belongs in infra because it has no
// project-specific knowledge: the same Levenshtein code could ship as a
// stand-alone library.
package text

import (
	"sort"
	"strings"
)

// CloseMatches returns up to n entries from choices that are similar to
// target, ordered by similarity descending. Comparison is case-insensitive.
// Matches must score at least 0.6 (where 1.0 is identical).
func CloseMatches(target string, choices []string, n int) []string {
	const threshold = 0.6
	type scored struct {
		s     string
		score float64
	}
	tl := strings.ToLower(target)
	var ranked []scored
	for _, c := range choices {
		score := Similarity(tl, strings.ToLower(c))
		if score >= threshold {
			ranked = append(ranked, scored{c, score})
		}
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if len(ranked) > n {
		ranked = ranked[:n]
	}
	result := make([]string, len(ranked))
	for i, r := range ranked {
		result[i] = r.s
	}
	return result
}

// Similarity returns a 0-1 score based on Levenshtein distance normalized
// by the longer string's length. Identical strings score 1.0; entirely
// different strings score 0.
func Similarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	d := Levenshtein(a, b)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	return 1.0 - float64(d)/float64(maxLen)
}

// Levenshtein returns the edit distance between a and b. Operates on runes,
// so unicode characters count as one edit each. O(len(a)*len(b)) time,
// O(len(b)) space.
func Levenshtein(a, b string) int {
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
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
