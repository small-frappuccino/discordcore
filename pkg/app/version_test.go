package app_test

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/app"
)

func TestAppVersion(t *testing.T) {
	orig := app.AppVersion()
	t.Cleanup(func() {
		app.SetAppVersion(orig)
	})

	app.SetAppVersion("v1.2.3-test")
	if got := app.AppVersion(); got != "v1.2.3-test" {
		t.Errorf("expected AppVersion 'v1.2.3-test', got %q", got)
	}
}
