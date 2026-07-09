package mutant

import "testing"

func TestIsMutant(t *testing.T) {
	tests := []struct {
		name string
		dna  []string
		want bool
	}{
		{
			// Reference mutant vector from requirements §3 / the PDF example.
			name: "pdf mutant example",
			dna:  []string{"ATGCGA", "CAGTGC", "TTATGT", "AGAAGG", "CCCCTA", "TCACTG"},
			want: true,
		},
		{
			// Verified human: no two sequences anywhere.
			name: "verified human",
			dna:  []string{"ATGC", "GCAT", "TACG", "CGTA"},
			want: false,
		},
		{
			// Exactly one sequence (top row) is not enough (FR-1.4).
			name: "single horizontal sequence is not mutant",
			dna:  []string{"AAAA", "GCAT", "TACG", "CGTA"},
			want: false,
		},
		{
			// Exactly 3 bases (not 4) is below the threshold and should return false.
			name: "three in a sequence is not mutant",
			dna:  []string{"AAAG", "GCAT", "TACG", "CGTA"},
			want: false,
		},
		{
			// FR-1.5 overlap: one physical run of five equal bases is two
			// overlapping windows, which alone makes the subject a mutant.
			name: "overlapping run of five counts twice",
			dna:  []string{"AAAAA", "GCATG", "TACGT", "CGTAC", "TGCAT"},
			want: true,
		},
		{
			// Vertical detection in isolation: two full columns of one base.
			name: "two vertical sequences",
			dna:  []string{"ACGT", "ACGT", "ACGT", "ACGT"},
			want: true,
		},
		{
			// ↘ diagonal detection: two parallel down-right diagonals of A's,
			// starting at (0,0) and (0,1); filler avoids other runs.
			name: "two down-right diagonals",
			dna:  []string{"AACGT", "CAAGT", "GCAAT", "TGCAA", "CGTGC"},
			want: true,
		},
		{
			// ↙ diagonal detection: two parallel down-left diagonals of A's,
			// starting at (0,3) and (0,4); filler avoids other runs.
			name: "two down-left diagonals",
			dna:  []string{"TGCAA", "TGAAG", "CAAGT", "AACGT", "CGTGC"},
			want: true,
		},
		{
			// Exactly one vertical sequence (col 0 = AAAA) and nothing else:
			// pins the FR-1.4 threshold for the vertical orientation.
			name: "single vertical sequence is not mutant",
			dna:  []string{"ACGT", "AGTC", "ATCG", "ACGT"},
			want: false,
		},
		{
			// Exactly one ↘ diagonal ((0,0)…(3,3) = AAAA) and nothing else:
			// pins the FR-1.4 threshold for the down-right orientation.
			name: "single down-right diagonal is not mutant",
			dna:  []string{"ACGT", "CACG", "GCAC", "TGCA"},
			want: false,
		},
		{
			// Ragged input whose scan actually forces an out-of-bounds column
			// lookahead: at (0,0) the ↘ step reads row1[1] of the 1-char row
			// "A". This exercises runFrom's bounds guard and asserts no panic +
			// false, unlike a case that early-exits at (0,0) (FR-1.7).
			name: "ragged forces out-of-bounds lookahead",
			dna:  []string{"ATCG", "A", "T", "C"},
			want: false,
		},
		{
			// N<4 can never be a mutant (FR-1.7) — matches requirements §3.
			name: "too small is not mutant",
			dna:  []string{"ATG", "CAG", "TTA"},
			want: false,
		},
		{
			name: "single row too small",
			dna:  []string{"AAAA"},
			want: false,
		},
		{
			name: "empty input",
			dna:  []string{},
			want: false,
		},
		{
			name: "nil input",
			dna:  nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsMutant(tc.dna); got != tc.want {
				t.Fatalf("IsMutant(%q) = %v, want %v", tc.dna, got, tc.want)
			}
		})
	}
}

// TestIsMutantRaggedDoesNotPanic ensures the bounds checks in runFrom hold for
// non-square input. The API rejects ragged matrices with 400, but the pure
// algorithm must still never panic (FR-1.7).
func TestIsMutantRaggedDoesNotPanic(t *testing.T) {
	ragged := [][]string{
		{"AAAA", "AA", "AAAA", "AAAA"}, // short interior rows
		{"AAAA", "AAAA", "AAAA", "A"},  // short final row
		{"", "", "", ""},               // empty rows
	}
	for _, dna := range ragged {
		// A panic here fails the test; the boolean result is unimportant.
		_ = IsMutant(dna)
	}
}
