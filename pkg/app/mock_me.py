import re

# 1. Update bot_runtime.go
with open('bot_runtime.go', 'r', encoding='utf-8') as f:
    content = f.read()

var_block = """var (
	// Test hook: override this in tests to prevent real websocket connections
	openBotArikawaState = func(ctx context.Context, s *state.State) error { return s.Open(ctx) }
	// Test hook: override this in tests to prevent real REST API calls
	fetchBotArikawaMe   = func(s *state.State) (*discord.User, error) { return s.Me() }
)"""

content = re.sub(r'var \(\n\t// Test hook: override this in tests to prevent real websocket connections\n\topenBotArikawaState = func\(ctx context\.Context, s \*state\.State\) error \{ return s\.Open\(ctx\) \}\n\)', var_block, content)

content = content.replace('me, err := arikawaState.Me()', 'me, err := fetchBotArikawaMe(arikawaState)')

with open('bot_runtime.go', 'w', encoding='utf-8') as f:
    f.write(content)

# 2. Update bot_runtime_test.go
with open('bot_runtime_test.go', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('// legacySession: sess,', 'legacySession: session.NewEmptySessionForCompat("Bot fake"),')

mock_me_code = """	origFetchBotArikawaMe := fetchBotArikawaMe
	t.Cleanup(func() {
		fetchBotArikawaMe = origFetchBotArikawaMe
	})
	fetchBotArikawaMe = func(s *state.State) (*discord.User, error) {
		return &discord.User{ID: 123, Username: "test"}, nil
	}
"""

if 'origFetchBotArikawaMe' not in content:
    content = content.replace('origOpenBotArikawaState := openBotArikawaState\n', mock_me_code + '	origOpenBotArikawaState := openBotArikawaState\n')

with open('bot_runtime_test.go', 'w', encoding='utf-8') as f:
    f.write(content)

# 3. Update bot_supervisor_test.go
with open('bot_supervisor_test.go', 'r', encoding='utf-8') as f:
    content = f.read()

if 'origFetchBotArikawaMe' not in content:
    content = content.replace('origOpenBotArikawaState := openBotArikawaState\n', mock_me_code + '	origOpenBotArikawaState := openBotArikawaState\n')

with open('bot_supervisor_test.go', 'w', encoding='utf-8') as f:
    f.write(content)

# 4. Update runner_test.go
with open('runner_test.go', 'r', encoding='utf-8') as f:
    content = f.read()

if 'origFetchBotArikawaMe' not in content:
    content = content.replace('origOpenBotArikawaState := openBotArikawaState\n', mock_me_code + '	origOpenBotArikawaState := openBotArikawaState\n')

with open('runner_test.go', 'w', encoding='utf-8') as f:
    f.write(content)

print("Fixed")
