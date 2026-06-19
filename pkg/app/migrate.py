import re

def migrate_bot_runtime(path):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()

    # Struct fields
    content = re.sub(r'session\s+\*discordgo\.Session', 'legacySession *session.LegacySession\n\tarikawaState  *state.State', content)

    # openBotRuntime body
    # Instead of full regex, let's just replace the session initialization
    init_old = '''	botToken := files.Unseal(instance.Token)
	sess, err := newDiscordSessionWithIntents(botToken, capabilities.intents)
	if err != nil {
		errWrap := fmt.Errorf("create discord session for %s: %w", instance.ID, err)
		log.EmitBlockingError("Blocking structural failure creating discord session", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := openBotDiscordSession(ctx, sess); err != nil {
		errWrap := fmt.Errorf("open discord session for %s: %w", instance.ID, err)
		log.EmitBlockingError("Blocking structural failure during socket bind and handshake", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	return &botRuntime{
		instanceID: instance.ID,
		session:    sess,
	}, nil'''

    init_new = '''	botToken := files.Unseal(instance.Token)
	arikawaState := state.New("Bot " + botToken)
	arikawaState.AddIntents(gateway.Intents(capabilities.intents))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := openBotArikawaState(ctx, arikawaState); err != nil {
		errWrap := fmt.Errorf("open discord session for %s: %w", instance.ID, err)
		log.EmitBlockingError("Blocking structural failure during socket bind and handshake", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	return &botRuntime{
		instanceID:   instance.ID,
		legacySession: session.NewEmptySessionForCompat(botToken),
		arikawaState: arikawaState,
	}, nil'''

    if init_old in content:
        content = content.replace(init_old, init_new)
    else:
        print("WARNING: init_old not found in bot_runtime.go")

    # shutdownBotRuntime body
    shutdown_old = '''	if runtime.session != nil {
		if err := runtime.session.Close(); err != nil {
			slog.Warn("Mitigated degradation: Socket teardown encountered an error",
				slog.String("botInstanceID", runtime.instanceID),
				slog.String("error", err.Error()),
			)
		}
	}'''

    shutdown_new = '''	if runtime.arikawaState != nil {
		if err := runtime.arikawaState.Close(); err != nil {
			errStr := err.Error()
			if !strings.Contains(errStr, "Session is closed") {
				slog.Warn("Mitigated degradation: Socket teardown encountered an error",
					slog.String("botInstanceID", runtime.instanceID),
					slog.String("error", errStr),
				)
			}
		}
	}'''
    
    if shutdown_old in content:
        content = content.replace(shutdown_old, shutdown_new)
    else:
        print("WARNING: shutdown_old not found in bot_runtime.go")

    # Re-add variables and imports
    if 'openBotArikawaState =' not in content:
        content = content.replace(')\n\nimport (', ')\n\nvar (\n\t// Test hook: override this in tests to prevent real websocket connections\n\topenBotArikawaState = func(ctx context.Context, s *state.State) error { return s.Open(ctx) }\n)\n\nimport (')
        content = content.replace('"github.com/diamondburned/arikawa/v3/state"\n)', '"github.com/diamondburned/arikawa/v3/state"\n\t"github.com/small-frappuccino/discordcore/pkg/discord/session"\n)')

    content = content.replace('listBotGuildIDsFromSessionState(runtime.session)', 'listBotGuildIDsFromSessionState(runtime.legacySession)')
    content = content.replace('runtime.session != nil', 'runtime.legacySession != nil')
    content = content.replace('runtime.session,', 'runtime.legacySession,')

    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)

migrate_bot_runtime(r'd:\Users\alice\git\discordcore\pkg\app\bot_runtime.go')

def migrate_bot_runtime_test(path):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()

    content = content.replace('capabilities.intents & gateway.IntentGuildMessageReactions', 'capabilities.intents & int(gateway.IntentGuildMessageReactions)')
    content = content.replace('legacySession:', '// legacySession:')

    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)

migrate_bot_runtime_test(r'd:\Users\alice\git\discordcore\pkg\app\bot_runtime_test.go')

print("Migration applied")
