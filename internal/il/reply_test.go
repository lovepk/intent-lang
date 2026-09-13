package il

import "testing"

const replyTestArchive = `INTENT calc@1.0.0
KIND program
FIDELITY behavior
TARGET python

CONTRACT
  R1: add(a,b) -> a+b

ACCEPT
  A1: add(1,2) == 3

OPEN
  ?1: 取整方式   default: 向下取整
`

func TestCheckReplyClaims(t *testing.T) {
	cases := []struct {
		name     string
		reply    string
		declared []string
		wantSev  []string
		wantIDs  []string
	}{
		{
			name:    "phantom id is an error",
			reply:   "我已新增 R9 处理负数。",
			wantSev: []string{"error"},
			wantIDs: []string{"R9"},
		},
		{
			name:     "declared change verb is clean",
			reply:    "本次修改了 R1 的加法规则。",
			declared: []string{"R1"},
		},
		{
			name:    "undeclared change claim is a suggestion",
			reply:   "我新增了 A1 用例。",
			wantSev: []string{"suggestion"},
			wantIDs: []string{"A1"},
		},
		{
			name:  "plain reference without change verb is ignored",
			reply: "根据 R1 的规则，输入按此处理。",
		},
		{
			name:  "open id reference without change verb is ignored",
			reply: "取整按 ?1 的默认处理。",
		},
		{
			name:  "empty reply yields nothing",
			reply: "",
		},
		{
			name:     "adjacent ids separated by punctuation are both found",
			reply:    "新增 R1，A1。",
			declared: []string{"R1", "A1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckReplyClaims(tc.reply, replyTestArchive, tc.declared)
			if len(got) != len(tc.wantSev) {
				t.Fatalf("got %d findings %+v, want %d", len(got), got, len(tc.wantSev))
			}
			for i := range got {
				if got[i].Severity != tc.wantSev[i] {
					t.Errorf("finding %d severity = %q, want %q", i, got[i].Severity, tc.wantSev[i])
				}
				if got[i].ID != tc.wantIDs[i] {
					t.Errorf("finding %d id = %q, want %q", i, got[i].ID, tc.wantIDs[i])
				}
			}
		})
	}
}
