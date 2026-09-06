package main

import "testing"

func TestPositionalArg(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"c-123", "--repo", ".x"}, "c-123"},
		{[]string{"--repo", ".x"}, ""},
		{[]string{"--repo", ".x", "c-9"}, "c-9"},
		{[]string{"c-9", "--repo", ".x", "--id", "c-8"}, "c-9"},
		{[]string{}, ""},
	}
	for _, c := range cases {
		if got := positionalArg(c.args); got != c.want {
			t.Errorf("positionalArg(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}
