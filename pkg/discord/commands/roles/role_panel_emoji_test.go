package roles

import "testing"

func TestParseRolePanelButtonEmoji(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		input        string
		wantName     string
		wantID       string
		wantAnimated bool
		wantErr      bool
	}{
		{name: "empty returns blanks", input: ""},
		{name: "trims whitespace", input: "   "},
		{
			name:     "unicode glyph",
			input:    "👋",
			wantName: "👋",
		},
		{
			name:     "custom static emoji",
			input:    "<:clouud:1378934415186464808>",
			wantName: "clouud",
			wantID:   "1378934415186464808",
		},
		{
			name:         "custom animated emoji",
			input:        "<a:flame:1378934415186464808>",
			wantName:     "flame",
			wantID:       "1378934415186464808",
			wantAnimated: true,
		},
		{
			name:    "malformed bracketed input",
			input:   "<:missing>",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			name, id, animated, err := parseRolePanelButtonEmoji(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tc.wantName || id != tc.wantID || animated != tc.wantAnimated {
				t.Fatalf("unexpected parse for %q: name=%q id=%q animated=%v", tc.input, name, id, animated)
			}
		})
	}
}
