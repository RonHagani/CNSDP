package auth

import "testing"

func TestBearer(t *testing.T) {
	const token = "secret-token"
	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{"correct token", "Bearer secret-token", true},
		{"incorrect token", "Bearer wrong-token", false},
		{"missing header", "", false},
		{"malformed prefix", "secret-token", false},
		{"wrong scheme", "Basic secret-token", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Bearer(c.header, token); got != c.want {
				t.Errorf("Bearer(%q, ...) = %v, want %v", c.header, got, c.want)
			}
		})
	}
}
