package qotd

import (
	"testing"
)

func TestPublishNowParamsShouldConsumeAutomaticSlotDefaultsToTrue(t *testing.T) {
	t.Parallel()

	truthy := true
	falsy := false

	cases := []struct {
		name string
		in   PublishNowParams
		want bool
	}{
		{name: "nil pointer defaults to consuming the slot", in: PublishNowParams{}, want: true},
		{name: "explicit true keeps slot consumption", in: PublishNowParams{ConsumeAutomaticSlot: &truthy}, want: true},
		{name: "explicit false suppresses slot consumption", in: PublishNowParams{ConsumeAutomaticSlot: &falsy}, want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.in.ShouldConsumeAutomaticSlot(); got != tc.want {
				t.Fatalf("ShouldConsumeAutomaticSlot() = %v, want %v", got, tc.want)
			}
		})
	}
}
