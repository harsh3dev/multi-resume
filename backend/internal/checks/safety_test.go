package checks

import "testing"

func TestCheckNumbers(t *testing.T) {
	cases := []struct {
		orig, new string
		want      bool
	}{
		{"reduced latency by 34%", "cut latency by 34%", true},
		{"reduced latency by 34%", "improved latency by 99%", false}, // invented
		{"served 2M requests in 2024", "served 2M requests", true},
		{"no numbers", "zero numbers here", true},
		{"team of 5", "team of 15", false},
		{"2020 -- 2021", "2020 to 2021", true},
	}
	for _, tc := range cases {
		if got := CheckNumbers(tc.orig, tc.new); got != tc.want {
			t.Errorf("CheckNumbers(%q, %q) = %v, want %v", tc.orig, tc.new, got, tc.want)
		}
	}
}

func TestCheckNewClaims(t *testing.T) {
	corpus := "Built APIs with Go, Python, Redis and PostgreSQL at Acme in 2023."
	cases := []struct {
		new  string
		want int
	}{
		{"Built APIs with Go and Redis", 0},
		{"Deployed services on Kubernetes", 1}, // Kubernetes not in corpus
		{"Used Go, Python, and Docker", 1},     // Docker invented
		{"Improved latency at Acme", 0},
	}
	for _, tc := range cases {
		got := CheckNewClaims(corpus, tc.new)
		if len(got) != tc.want {
			t.Errorf("CheckNewClaims(%q) = %v, want %d", tc.new, got, tc.want)
		}
	}
}

func TestCheckMaxLen(t *testing.T) {
	if !CheckMaxLen("short", 10) {
		t.Fatal("short text rejected")
	}
	if CheckMaxLen("this is much too long", 5) {
		t.Fatal("long text accepted")
	}
}
