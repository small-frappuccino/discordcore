import re
import os

def migrate_bot_runtime(path):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()

    # 1. Imports
    if 'github.com/diamondburned/arikawa/v3/gateway' not in content:
        content = content.replace('"github.com/small-frappuccino/discordgo"\n', '')
        content = content.replace('golang.org/x/sync/errgroup"\n)', 'golang.org/x/sync/errgroup"\n\t"github.com/diamondburned/arikawa/v3/gateway"\n\t"github.com/diamondburned/arikawa/v3/state"\n\t"github.com/small-frappuccino/discordcore/pkg/discord/session"\n)')

    # 2. Add openBotArikawaState variable
    if 'openBotArikawaState' not in content:
        content = content.replace(')\n\nimport (', ')\n\nvar (\n\t// Test hook: override this in tests to prevent real websocket connections\n\topenBotArikawaState = func(ctx context.Context, s *state.State) error { return s.Open(ctx) }\n)\n\nimport (')
    
    # 3. botRuntime struct
    content = re.sub(r'session\s+\*discordgo\.Session', 'legacySession *session.LegacySession\n\tarikawaState  *state.State', content)

    # 4. openBotRuntime method
    old_init = r'''	discordSession, err := newDiscordSessionWithIntents(instance.Token, capabilities.intents)
	if err != nil {
		errWrap := fmt.Errorf("create discord session for %s: %w", instance.ID, err)
		log.EmitBlockingError("Blocking structural failure during session initialization", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	if capabilities.botStatusText != "" {
		discordSession.Identify.Presence = discordgo.GatewayStatusUpdate{
			Game: discordgo.Activity{
				Name: capabilities.botStatusText,
				Type: discordgo.ActivityTypeGame,
			},
		}
	}

	if err := openBotDiscordSession(ctx, discordSession); err != nil {
		errWrap := fmt.Errorf("open discord session for %s: %w", instance.ID, err)
		log.EmitBlockingError("Blocking structural failure during socket bind and handshake", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	if discordSession.State == nil || discordSession.State.User == nil {
		errState := fmt.Errorf("discord session state not properly initialized for %s", instance.ID)
		log.EmitBlockingError("Blocking structural failure reading initial state", errState, log.GenerateRequestID())
		return nil, errState
	}

	slog.Info("Architectural state transition: Gateway connection established",
		slog.String("botInstanceID", instance.ID),
		slog.String("botUser", fmt.Sprintf("%s#%s", discordSession.State.User.Username, discordSession.State.User.Discriminator)),
	)

	return &botRuntime{
		instanceID:   instance.ID,
		session:      discordSession,
		capabilities: capabilities,
		cancel:       cancel,
	}, nil'''

    new_init = r'''	botToken := files.Unseal(instance.Token)
	arikawaState := state.New("Bot " + botToken)
	arikawaState.AddIntents(gateway.Intents(capabilities.intents))

	if capabilities.botStatusText != "" {
		arikawaState.Gateway.AddCustomData(gateway.UpdatePresenceCommand{
			Activities: []discord.Activity{{
				Name: capabilities.botStatusText,
				Type: discord.GameActivity,
			}},
		})
	}

	if err := openBotArikawaState(ctx, arikawaState); err != nil {
		errWrap := fmt.Errorf("open discord session for %s: %w", instance.ID, err)
		log.EmitBlockingError("Blocking structural failure during socket bind and handshake", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	me, err := arikawaState.Me()
	if err != nil {
		errState := fmt.Errorf("discord session state not properly initialized for %s: %w", instance.ID, err)
		log.EmitBlockingError("Blocking structural failure reading initial state", errState, log.GenerateRequestID())
		return nil, errState
	}

	slog.Info("Architectural state transition: Gateway connection established",
		slog.String("botInstanceID", instance.ID),
		slog.String("botUser", fmt.Sprintf("%s#%s", me.Username, me.Discriminator)),
	)

	return &botRuntime{
		instanceID:   instance.ID,
		legacySession: session.NewEmptySessionForCompat(botToken),
		arikawaState: arikawaState,
		capabilities: capabilities,
		cancel:       cancel,
	}, nil'''
    
    content = content.replace(old_init, new_init)

    # 5. ShutdownBotRuntime
    old_shutdown = r'''	if runtime.session != nil {
		if err := runtime.session.Close(); err != nil {
			slog.Warn("Mitigated degradation: Socket teardown encountered an error",
				slog.String("botInstanceID", runtime.instanceID),
				slog.String("error", err.Error()),
			)
		}
	}'''
    
    new_shutdown = r'''	if runtime.arikawaState != nil {
		if err := runtime.arikawaState.Close(); err != nil {
			errStr := err.Error()
			if !strings.Contains(errStr, "Session is closed") && !strings.Contains(errStr, "use of closed network connection") {
				slog.Warn("Mitigated degradation: Socket teardown encountered an error",
					slog.String("botInstanceID", runtime.instanceID),
					slog.String("error", errStr),
				)
			}
		}
	}'''
    content = content.replace(old_shutdown, new_shutdown)

    # 6. Global replacements
    content = content.replace('runtime.session', 'runtime.legacySession')
    content = content.replace('listBotGuildIDsFromSessionState(runtime.legacySession)', 'listBotGuildIDsFromSessionState(runtime.legacySession)')
    content = content.replace('sessionForGuild(guildID string, feature string) (*discordgo.Session, error)', 'sessionForGuild(guildID string, feature string) (*session.LegacySession, error)')
    content = content.replace('discordSession, err := newDiscordSessionWithIntents(instance.Token, capabilities.intents)', '')
    content = content.replace('listBotGuildBindingsFromSessionState(botInstanceID string, session *discordgo.Session)', 'listBotGuildBindingsFromSessionState(botInstanceID string, session *session.LegacySession)')

    # Add back discord if missing
    if 'github.com/diamondburned/arikawa/v3/discord' not in content:
         content = content.replace('github.com/diamondburned/arikawa/v3/gateway', 'github.com/diamondburned/arikawa/v3/discord"\n\t"github.com/diamondburned/arikawa/v3/gateway')

    # Remove the mock variables we just wiped from tests
    content = re.sub(r'var openBotDiscordSession = session.OpenSession\n', '', content)
    content = re.sub(r'var newDiscordSessionWithIntents = session.NewDiscordSessionWithIntents\n', '', content)
    content = re.sub(r'var newDiscordSession = session.NewDiscordSession\n', '', content)

    # Handle AddHandler replacements that were legacy
    content = re.sub(r'runtime\.legacySession\.AddHandler\(func\(s \*discordgo\.Session, e \*discordgo\.GuildCreate\) \{.*?\n\t\t\}\)\n', '', content, flags=re.DOTALL)
    content = re.sub(r'runtime\.legacySession\.AddHandler\(func\(s \*discordgo\.Session, e \*discordgo\.GuildDelete\) \{.*?\n\t\t\}\)\n', '', content, flags=re.DOTALL)
    
    # Actually AddHandler for GuildCreate and GuildDelete needs to be replaced with arikawa handlers
    # But since we just want to remove discordgo, if they were used for cache we can bind them to arikawaState.AddHandler.
    
    # Wait, the handlers were:
    # runtime.session.AddHandler(func(s *discordgo.Session, e *discordgo.GuildCreate) { runtime.unifiedCache.SetGuildAvailable(e.Guild.ID, true) })
    # runtime.session.AddHandler(func(s *discordgo.Session, e *discordgo.GuildDelete) { runtime.unifiedCache.SetGuildAvailable(e.Guild.ID, false) })
    
    content = content.replace('runtime.legacySession.AddHandler(func(s *discordgo.Session, e *discordgo.GuildCreate) {\n\t\t\truntime.unifiedCache.SetGuildAvailable(e.Guild.ID, true)\n\t\t})', 'runtime.arikawaState.AddHandler(func(e *gateway.GuildCreateEvent) {\n\t\t\truntime.unifiedCache.SetGuildAvailable(e.ID.String(), true)\n\t\t})')
    content = content.replace('runtime.legacySession.AddHandler(func(s *discordgo.Session, e *discordgo.GuildDelete) {\n\t\t\truntime.unifiedCache.SetGuildAvailable(e.Guild.ID, false)\n\t\t})', 'runtime.arikawaState.AddHandler(func(e *gateway.GuildDeleteEvent) {\n\t\t\truntime.unifiedCache.SetGuildAvailable(e.ID.String(), false)\n\t\t})')

    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)

migrate_bot_runtime(r'd:\Users\alice\git\discordcore\pkg\app\bot_runtime.go')
print("Migrated bot_runtime.go")
