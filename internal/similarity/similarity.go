package similarity

import (
	"fmt"
	"strings"
)

// Report summarizes the comparison between a reference artifact and a
// reproduced artifact.
type Report struct {
	Ref     string
	Cand    string
	LinesA  int
	LinesB  int
	LCS     int
	Added   int
	Removed int
	Hunks   []Hunk
}

type Hunk struct {
	Side   string // "ref-only" | "cand-only"
	LineNo int
	Text   string
}

// Score returns a normalized similarity in [0,1] based on the LCS line count
// (Dice-like coefficient: 2*LCS/(linesA+linesB)).
func (r Report) Score() float64 {
	den := r.LinesA + r.LinesB
	if den == 0 {
		return 1
	}
	return float64(2*r.LCS) / float64(den)
}

// Pass reports whether the score meets the threshold.
func (r Report) Pass(threshold float64) bool {
	return r.Score() >= threshold
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, l := range lines {
		l = strings.TrimRight(l, "\r")
		out = append(out, l)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func Compare(ref, cand string) Report {
	a := splitLines(ref)
	b := splitLines(cand)

	n, m := len(a), len(b)
	// LCS DP. Guard against very large inputs by capping to the first 4000 lines.
	if n > 4000 {
		a = a[:4000]
		n = 4000
	}
	if m > 4000 {
		b = b[:4000]
		m = 4000
	}

	// Use two-row DP to bound memory; O(n*m).
	prev := make([]int, m+1)
	cur := make([]int, m+1)
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else if prev[j] >= cur[j-1] {
				cur[j] = prev[j]
			} else {
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	lcs := prev[m]

	// Diff hunks: walk the DP table.
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack collects edit ops.
	type op struct {
		kind   string
		ia, ib int
	}
	ops := make([]op, 0, n+m)
	i, j := n, m
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			ops = append(ops, op{"same", i - 1, j - 1})
			i--
			j--
		} else if dp[i-1][j] >= dp[i][j-1] {
			ops = append(ops, op{"del", i - 1, -1})
			i--
		} else {
			ops = append(ops, op{"add", -1, j - 1})
			j--
		}
	}
	for i > 0 {
		ops = append(ops, op{"del", i - 1, -1})
		i--
	}
	for j > 0 {
		ops = append(ops, op{"add", -1, j - 1})
		j--
	}
	// reverse
	for x, y := 0, len(ops)-1; x < y; x, y = x+1, y-1 {
		ops[x], ops[y] = ops[y], ops[x]
	}

	r := Report{
		LinesA: n,
		LinesB: m,
		LCS:    lcs,
	}
	seen := make(map[Hunk]bool)
	for _, o := range ops {
		switch o.kind {
		case "del":
			h := Hunk{Side: "ref-only", LineNo: o.ia + 1, Text: a[o.ia]}
			if !seen[h] {
				r.Hunks = append(r.Hunks, h)
				seen[h] = true
			}
			r.Removed++
		case "add":
			h := Hunk{Side: "cand-only", LineNo: o.ib + 1, Text: b[o.ib]}
			if !seen[h] {
				r.Hunks = append(r.Hunks, h)
				seen[h] = true
			}
			r.Added++
		}
	}
	// trim to a readable window
	if len(r.Hunks) > 40 {
		r.Hunks = r.Hunks[:40]
	}
	return r
}

func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "相似度: %.3f (ref %d 行, cand %d 行, 公共 %d 行, ref独有 %d, cand独有 %d)\n",
		r.Score(), r.LinesA, r.LinesB, r.LCS, r.Removed, r.Added)
	if len(r.Hunks) == 0 {
		b.WriteString("无差异\n")
		return b.String()
	}
	for _, h := range r.Hunks {
		marker := "-"
		if h.Side == "cand-only" {
			marker = "+"
		}
		fmt.Fprintf(&b, "  %s %4d %s\n", marker, h.LineNo, h.Text)
	}
	return b.String()
}
