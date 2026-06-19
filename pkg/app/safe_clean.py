def replace_exact(path, blocks):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()
    for block in blocks:
        if block in content:
            content = content.replace(block, "")
        else:
            print(f"WARNING: block not found in {path}")
    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)

bot_supervisor_blocks = [
    '\torigNewDiscordSession := newDiscordSession\n\torigNewDiscordSessionWithIntents := newDiscordSessionWithIntents\n\torigOpenBotDiscordSession := openBotDiscordSession\n',
    '\t\tnewDiscordSession = origNewDiscordSession\n\t\tnewDiscordSessionWithIntents = origNewDiscordSessionWithIntents\n\t\topenBotDiscordSession = origOpenBotDiscordSession\n',
    '\tsession1, _ := discordgo.New("Bot token1")\n\tsession1.State.User = &discordgo.User{ID: "child1"}\n\tsession2, _ := discordgo.New("Bot token2")\n\tsession2.State.User = &discordgo.User{ID: "child2"}\n\tsession3, _ := discordgo.New("Bot token3")\n\tsession3.State.User = &discordgo.User{ID: "child3"}\n\n\tnewDiscordSession = func(token string) (*discordgo.Session, error) {\n\t\tif token == "token1" {\n\t\t\treturn session1, nil\n\t\t}\n\t\tif token == "token2" {\n\t\t\treturn session2, nil\n\t\t}\n\t\tif token == "token3" {\n\t\t\treturn session3, nil\n\t\t}\n\t\treturn nil, errors.New("unknown token")\n\t}\n\tnewDiscordSessionWithIntents = func(token string, _ discordgo.Intent) (*discordgo.Session, error) {\n\t\tif token == "token1" {\n\t\t\treturn session1, nil\n\t\t}\n\t\tif token == "token2" {\n\t\t\treturn session2, nil\n\t\t}\n\t\tif token == "token3" {\n\t\t\treturn session3, nil\n\t\t}\n\t\treturn nil, errors.New("unknown token")\n\t}\n\topenBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error {\n\t\treturn nil\n\t}\n',
    '\tnewDiscordSession = func(token string) (*discordgo.Session, error) {\n\t\ts, _ := discordgo.New(token)\n\t\ts.State.User = &discordgo.User{ID: token}\n\t\treturn s, nil\n\t}\n\tnewDiscordSessionWithIntents = func(token string, _ discordgo.Intent) (*discordgo.Session, error) {\n\t\ts, _ := discordgo.New(token)\n\t\ts.State.User = &discordgo.User{ID: token}\n\t\treturn s, nil\n\t}\n\topenBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error {\n\t\treturn nil\n\t}\n',
    '\tsession1, _ := discordgo.New("token1")\n\tsession1.State.User = &discordgo.User{ID: "child1"}\n\n\tnewDiscordSession = func(token string) (*discordgo.Session, error) {\n\t\treturn session1, nil\n\t}\n\tnewDiscordSessionWithIntents = func(token string, _ discordgo.Intent) (*discordgo.Session, error) {\n\t\treturn session1, nil\n\t}\n\topenBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error {\n\t\treturn nil\n\t}\n',
    '\t"github.com/small-frappuccino/discordgo"\n'
]
replace_exact(r'd:\Users\alice\git\discordcore\pkg\app\bot_supervisor_test.go', bot_supervisor_blocks)

runner_blocks = [
    '\tsession, err := discordgo.New("Bot test-token")\n\tif err != nil {\n\t\tt.Fatalf("create fake discord session: %v", err)\n\t}\n\tsession.State.User = &discordgo.User{\n\t\tID:            "bot-id",\n\t\tUsername:      "testuser",\n\t\tDiscriminator: "0001",\n\t\tBot:           true,\n\t}\n\tsession.State.Guilds = []*discordgo.Guild{{ID: "guild-1"}}\n\n\torigNewDiscordSession := newDiscordSession\n\torigNewDiscordSessionWithIntents := newDiscordSessionWithIntents\n',
    '\t\tnewDiscordSession = origNewDiscordSession\n\t\tnewDiscordSessionWithIntents = origNewDiscordSessionWithIntents\n',
    '\tnewDiscordSession = func(string) (*discordgo.Session, error) {\n\t\treturn session, nil\n\t}\n\tnewDiscordSessionWithIntents = func(string, discordgo.Intent) (*discordgo.Session, error) {\n\t\treturn session, nil\n\t}\n\torigOpenBotDiscordSession := openBotDiscordSession\n\tt.Cleanup(func() {\n\t\topenBotDiscordSession = origOpenBotDiscordSession\n\t})\n\topenBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error { return nil }\n',
    '\t"github.com/small-frappuccino/discordgo"\n'
]
replace_exact(r'd:\Users\alice\git\discordcore\pkg\app\runner_test.go', runner_blocks)

# We removed origNewDiscordSession etc, so we should clean up the variable assignments.
# Oh wait, `origNewDiscordSession` is at the top of `origShutdownDelay := shutdownDelay`, so I removed too much or too little?
# Wait! I removed `origNewDiscordSession` but not `newDiscordSession = orig...`? I did in runner_blocks.
# But `runner_test.go` has multiple occurrences of this!
print("Done")
