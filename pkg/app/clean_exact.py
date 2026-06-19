def replace_exact(path, blocks):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read().replace('\r\n', '\n')
    for block in blocks:
        if block in content:
            content = content.replace(block, "")
        else:
            print(f"WARNING: block not found in {path}")
    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)

bot_supervisor_blocks = [
    r'''	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origOpenBotDiscordSession := openBotDiscordSession
''',
    r'''		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		openBotDiscordSession = origOpenBotDiscordSession
''',
    r'''	session1, _ := discordgo.New("Bot token1")
	session1.State.User = &discordgo.User{ID: "child1"}
	session2, _ := discordgo.New("Bot token2")
	session2.State.User = &discordgo.User{ID: "child2"}
	session3, _ := discordgo.New("Bot token3")
	session3.State.User = &discordgo.User{ID: "child3"}

	newDiscordSession = func(token string) (*discordgo.Session, error) {
		if token == "token1" {
			return session1, nil
		}
		if token == "token2" {
			return session2, nil
		}
		if token == "token3" {
			return session3, nil
		}
		return nil, errors.New("unknown token")
	}
	newDiscordSessionWithIntents = func(token string, _ discordgo.Intent) (*discordgo.Session, error) {
		if token == "token1" {
			return session1, nil
		}
		if token == "token2" {
			return session2, nil
		}
		if token == "token3" {
			return session3, nil
		}
		return nil, errors.New("unknown token")
	}
	openBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error {
		return nil
	}
''',
    r'''	newDiscordSession = func(token string) (*discordgo.Session, error) {
		s, _ := discordgo.New(token)
		s.State.User = &discordgo.User{ID: token}
		return s, nil
	}
	newDiscordSessionWithIntents = func(token string, _ discordgo.Intent) (*discordgo.Session, error) {
		s, _ := discordgo.New(token)
		s.State.User = &discordgo.User{ID: token}
		return s, nil
	}
	openBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error {
		return nil
	}
''',
    r'''	session1, _ := discordgo.New("token1")
	session1.State.User = &discordgo.User{ID: "child1"}

	newDiscordSession = func(token string) (*discordgo.Session, error) {
		return session1, nil
	}
	newDiscordSessionWithIntents = func(token string, _ discordgo.Intent) (*discordgo.Session, error) {
		return session1, nil
	}
	openBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error {
		return nil
	}
''',
    r'''	"github.com/small-frappuccino/discordgo"
'''
]
replace_exact('bot_supervisor_test.go', bot_supervisor_blocks)

runner_blocks = [
    r'''	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create fake discord session: %v", err)
	}
	session.State.User = &discordgo.User{
		ID:            "bot-id",
		Username:      "testuser",
		Discriminator: "0001",
		Bot:           true,
	}
	session.State.Guilds = []*discordgo.Guild{{ID: "guild-1"}}

	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
''',
    r'''		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
''',
    r'''	newDiscordSession = func(string) (*discordgo.Session, error) {
		return session, nil
	}
	newDiscordSessionWithIntents = func(string, discordgo.Intent) (*discordgo.Session, error) {
		return session, nil
	}
	origOpenBotDiscordSession := openBotDiscordSession
	t.Cleanup(func() {
		openBotDiscordSession = origOpenBotDiscordSession
	})
	openBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error { return nil }
''',
    r'''	"github.com/small-frappuccino/discordgo"
'''
]
replace_exact('runner_test.go', runner_blocks)

# Notice in runner blocks I removed `session, err := ...`. So the later `err = Run(appName)` might fail if `err` isn't declared.
# I need to add a small replacement to fix that in runner_test.go:
with open('runner_test.go', 'r', encoding='utf-8') as f:
    content = f.read()
content = content.replace('err = Run(appName)', 'err := Run(appName)')
with open('runner_test.go', 'w', encoding='utf-8') as f:
    f.write(content)
