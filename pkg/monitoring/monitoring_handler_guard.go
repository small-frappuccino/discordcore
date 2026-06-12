package monitoring

import (
	"context"

	"github.com/small-frappuccino/discordgo"
)

// GuardedHandler creates a discordgo event handler that enforces the monitoring
// lifecycle bounds before calling fn. If the service is not running, fn is not called.
func GuardedHandler[E any](sl *ServiceLifecycle, fn func(context.Context, *discordgo.Session, *E)) func(*discordgo.Session, *E) {
	return func(s *discordgo.Session, e *E) {
		runCtx, done, ok := sl.Begin()
		if !ok {
			return
		}
		defer done()
		fn(runCtx, s, e)
	}
}
