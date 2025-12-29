package app

import "testing"

func TestFormatStartupMessage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		appName     string
		appVersion  string
		coreVersion string
		want        string
	}{
		{
			name:        "no app version includes discordcore",
			appName:     "alicebot",
			appVersion:  "",
			coreVersion: "v0.146.0",
			want:        "🚀 Starting alicebot (discordcore v0.146.0)...",
		},
		{
			name:        "different versions include both",
			appName:     "alicebot",
			appVersion:  "v0.114.0",
			coreVersion: "v0.146.0",
			want:        "🚀 Starting alicebot v0.114.0 (discordcore v0.146.0)...",
		},
		{
			name:        "same versions omit discordcore suffix",
			appName:     "alicebot",
			appVersion:  "v0.146.0",
			coreVersion: "v0.146.0",
			want:        "🚀 Starting alicebot v0.146.0...",
		},
		{
			name:        "trims spaces",
			appName:     " alicebot ",
			appVersion:  " v0.146.0 ",
			coreVersion: " v0.146.0 ",
			want:        "🚀 Starting alicebot v0.146.0...",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatStartupMessage(tc.appName, tc.appVersion, tc.coreVersion)
			if got != tc.want {
				t.Fatalf("formatStartupMessage() mismatch\nwant: %q\ngot:  %q", tc.want, got)
			}
		})
	}
}
