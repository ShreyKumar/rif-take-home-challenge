// Package mutant implements the core detection algorithm: given an N×N DNA
// matrix, decide whether the subject is a mutant.
//
// A "sequence" is four consecutive identical bases along a row, a column, or
// either diagonal (↘ and ↙). The subject is a mutant iff at least two such
// sequences exist (FR-1.4). Overlapping windows count independently, so a run
// of five identical bases yields two sequences (FR-1.5).
//
// The scan is O(N²): every cell is a candidate window start, and each of the
// four forward directions is a bounded, constant-length lookahead. Detection
// short-circuits the instant the second sequence is found (FR-1.6), and every
// access is bounds-checked so ragged or sub-4 input never panics (FR-1.7).
package mutant

// seqLen is the number of consecutive identical bases that form one sequence.
const seqLen = 4

// forward directions scanned from each cell: right, down, down-right (↘),
// and down-left (↙). Scanning only forward means each distinct window is
// counted exactly once, from its starting cell.
var directions = [...][2]int{
	{0, 1},  // horizontal →
	{1, 0},  // vertical ↓
	{1, 1},  // diagonal ↘
	{1, -1}, // diagonal ↙
}

// IsMutant reports whether the DNA matrix contains more than one sequence of
// four consecutive identical bases. It returns false for any matrix smaller
// than 4×4 (which cannot hold a sequence) and never panics on ragged input.
func IsMutant(dna []string) bool {
	n := len(dna)
	if n < seqLen {
		return false
	}

	found := 0
	for r := 0; r < n; r++ {
		row := dna[r]
		for c := 0; c < len(row); c++ {
			base := row[c]
			for _, d := range directions {
				if runFrom(dna, r, c, d[0], d[1], base) {
					found++
					if found >= 2 {
						return true
					}
				}
			}
		}
	}
	return false
}

// runFrom reports whether the seqLen cells starting at (r,c) and stepping by
// (dr,dc) are all equal to base. Every step is bounds-checked against the
// actual row length, so non-square matrices are handled without panicking.
func runFrom(dna []string, r, c, dr, dc int, base byte) bool {
	n := len(dna)
	for k := 1; k < seqLen; k++ {
		rr := r + dr*k
		cc := c + dc*k
		if rr >= n {
			return false
		}
		row := dna[rr]
		if cc < 0 || cc >= len(row) {
			return false
		}
		if row[cc] != base {
			return false
		}
	}
	return true
}
