package notetext

import "regexp"

var embedBlockPattern = regexp.MustCompile(`!\[\[.+?\]\]`)

// FindEmbedRanges returns the [start, end) byte positions of all ![[...]] blocks.
func FindEmbedRanges(content string) [][2]int {
	locs := embedBlockPattern.FindAllStringIndex(content, -1)
	ranges := make([][2]int, len(locs))
	for i, loc := range locs {
		ranges[i] = [2]int{loc[0], loc[1]}
	}
	return ranges
}

// InsideRanges returns true if position pos falls within any of the given ranges.
func InsideRanges(pos int, ranges [][2]int) bool {
	for _, r := range ranges {
		if pos >= r[0] && pos < r[1] {
			return true
		}
	}
	return false
}
