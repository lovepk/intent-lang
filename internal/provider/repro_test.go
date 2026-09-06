package provider

import "testing"

func TestStripFence(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"```python\nprint(1)\n```", "print(1)"},
		{"```\nprint(1)\n```", "print(1)"},
		{"print(1)", "print(1)"},
		{"```python\nprint(1)", "print(1)"},
	}
	for _, c := range cases {
		if got := StripFence(c.in); got != c.want {
			t.Errorf("StripFence(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
