import os

content = open('bot_runtime.go', 'r', encoding='utf-8').read()

# Fix `listBotGuildIDsFromSessionState`
content = content.replace('listBotGuildIDsFromSessionState(session)', 'listBotGuildIDsFromSessionState(session)')
# Actually I need to add back `listBotGuildIDsFromSessionState` if it's missing!
if 'func listBotGuildIDsFromSessionState' not in content:
    func_code = '''
func listBotGuildIDsFromSessionState(session *session.LegacySession) ([]string, error) {
	if session == nil || session.State == nil {
		return nil, errors.New("state unavailable")
	}
	session.State.RLock()
	defer session.State.RUnlock()
	out := make([]string, 0, len(session.State.Guilds))
	for _, g := range session.State.Guilds {
		out = append(out, g.ID)
	}
	return out, nil
}
'''
    content = content.replace('func listBotGuildBindingsFromSessionState', func_code + '\nfunc listBotGuildBindingsFromSessionState')

# Remove duplicate arikawaState
content = content.replace('\tarikawaState  *state.State\n\tarikawaState  *state.State', '\tarikawaState  *state.State')

# Replace `discordSession` with `legacySession` in any remaining places (if they aren't meant to be `arikawaState`):
# In my manual edit I wiped out `session:      discordSession,` and replaced it with `legacySession: session.NewEmptySessionForCompat(botToken), arikawaState: arikawaState,`
# Wait, `discordSession` is still undefined somewhere?
# "bot_runtime.go:642:3: undefined: discordSession" -> I already fixed it manually to `arikawaState.Gateway.AddCustomData`!
# "bot_runtime.go:650:12: undefined: openBotDiscordSession" -> Fixed manually!
# Wait! `bot_runtime.go:632:5: undefined: err`! That's what I fixed manually!
# I DID fix those manually, but then I ran `go build bot_runtime.go` which gave me:
# bot_runtime.go:553:14: undefined: listBotGuildIDsFromSessionState
# bot_runtime.go:581:28: undefined: StartupTaskOrchestrator
# bot_runtime.go:582:27: undefined: RunProfile
# bot_runtime.go:584:28: undefined: controlServerHolder
# ...
# The `undefined` types are from OTHER FILES in the `app` package! `go build bot_runtime.go` does NOT include `startup.go`, `control.go` etc!
# I MUST USE `go build .` or `go test .` instead of `go build bot_runtime.go`!

# So the ONLY REAL error was:
# bot_runtime.go:274:2: arikawaState redeclared
# bot_runtime.go:553:14: undefined: listBotGuildIDsFromSessionState

open('bot_runtime.go', 'w', encoding='utf-8').write(content)
print("Fixed")
