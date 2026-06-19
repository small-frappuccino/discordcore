import re

content = open('bot_runtime.go', 'r', encoding='utf-8').read()

# Fix the import syntax error
var_block = """var (
	// Test hook: override this in tests to prevent real websocket connections
	openBotArikawaState = func(ctx context.Context, s *state.State) error { return s.Open(ctx) }
)
"""
content = content.replace(")\n\nvar (\n\t// Test hook: override this in tests to prevent real websocket connections\n\topenBotArikawaState = func(ctx context.Context, s *state.State) error { return s.Open(ctx) }\n)", "")

if "var openBotArikawaState" not in content:
    content = content.replace(')\n\n// ErrSessionUnavailable', var_block + '\n// ErrSessionUnavailable')

open('bot_runtime.go', 'w', encoding='utf-8').write(content)

content_test = open('bot_runtime_test.go', 'r', encoding='utf-8').read()
content_test = content_test.replace('capabilities.intents & gateway.IntentGuildMessageReactions == 0', 'int(capabilities.intents) & int(gateway.IntentGuildMessageReactions) == 0')
content_test = content_test.replace('\tsess := session.NewEmptySessionForCompat("Bot fake")\n', '')
open('bot_runtime_test.go', 'w', encoding='utf-8').write(content_test)

print("Fixed")
