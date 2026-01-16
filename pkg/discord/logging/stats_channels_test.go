package logging

import "testing"

func TestNormalizeMemberType(t *testing.T) {
	cases := map[string]string{
		"":       "all",
		"ALL":    "all",
		"bots":   "bots",
		"Bot":    "bots",
		"humans": "humans",
		"HUMAN":  "humans",
		"weird":  "all",
	}

	for raw, want := range cases {
		if got := normalizeMemberType(raw); got != want {
			t.Fatalf("normalizeMemberType(%q) = %q; want %q", raw, got, want)
		}
	}
}

func TestMemberTypeMatches(t *testing.T) {
	tests := []struct {
		raw   string
		isBot bool
		want  bool
	}{
		{"", false, true},
		{"", true, true},
		{"bots", true, true},
		{"bots", false, false},
		{"humans", false, true},
		{"humans", true, false},
	}

	for _, tt := range tests {
		if got := memberTypeMatches(tt.raw, tt.isBot); got != tt.want {
			t.Fatalf("memberTypeMatches(%q, %v) = %v; want %v", tt.raw, tt.isBot, got, tt.want)
		}
	}
}

func TestRenderStatsChannelName(t *testing.T) {
	got := renderStatsChannelName("Total Proxies", "", 42)
	if got != "Total Proxies: 42" {
		t.Fatalf("default template: got %q", got)
	}

	got = renderStatsChannelName("", "", 7)
	if got != "7" {
		t.Fatalf("default template without label: got %q", got)
	}

	got = renderStatsChannelName("Bunny", "{label} | {count}", 3)
	if got != "Bunny | 3" {
		t.Fatalf("custom template: got %q", got)
	}
}
