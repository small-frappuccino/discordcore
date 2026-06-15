package control

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/sync/singleflight"
)

type accessibleGuildResolver struct {
	providerSource func() *discordOAuthProvider
	bindings       *botGuildBindingSource
	cache          *accessibleGuildCache
	evaluator      *guildAccessEvaluator
	sfg            singleflight.Group
}

func newAccessibleGuildResolver(
	providerSource func() *discordOAuthProvider,
	bindings *botGuildBindingSource,
	cache *accessibleGuildCache,
	evaluator *guildAccessEvaluator,
) *accessibleGuildResolver {
	return &accessibleGuildResolver{
		providerSource: providerSource,
		bindings:       bindings,
		cache:          cache,
		evaluator:      evaluator,
	}
}

// ResolveAccessibleGuilds resolves accessible guilds.
func (resolver *accessibleGuildResolver) ResolveAccessibleGuilds(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	return resolver.resolveWithOptions(ctx, session, true, true)
}

// ResolveAccessibleGuildsFresh resolves accessible guilds fresh.
func (resolver *accessibleGuildResolver) ResolveAccessibleGuildsFresh(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	return resolver.resolveWithOptions(ctx, session, false, false)
}

// ResolveAccessibleGuildsRefreshed resolves accessible guilds refreshed.
func (resolver *accessibleGuildResolver) ResolveAccessibleGuildsRefreshed(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	return resolver.resolveWithOptions(ctx, session, false, true)
}

// ResolveManageableGuilds resolves manageable guilds.
func (resolver *accessibleGuildResolver) ResolveManageableGuilds(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	accessible, err := resolver.ResolveAccessibleGuilds(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("accessibleGuildResolver.ResolveManageableGuilds: %w", err)
	}
	return filterAccessibleGuildsByLevel(accessible, guildAccessLevelWrite), nil
}

// InvalidateCache invalidates cache.
func (resolver *accessibleGuildResolver) InvalidateCache() {
	if resolver == nil || resolver.cache == nil {
		return
	}
	resolver.cache.InvalidateAll()
}

func (resolver *accessibleGuildResolver) resolveWithOptions(
	ctx context.Context,
	session discordOAuthSession,
	useCache bool,
	storeCache bool,
) ([]accessibleGuildResponse, error) {
	provider := resolver.provider()
	if provider == nil {
		return nil, errDiscordOAuthUnavailable
	}

	if useCache && resolver.cache != nil {
		if cached, ok := resolver.cache.Get(session.ID); ok {
			return resolver.materializeAccessibleGuilds(cached, session.User.ID), nil
		}
	}

	v, err, _ := resolver.sfg.Do(session.User.ID, func() (any, error) {
		botGuildSet, resolveErr := resolver.resolveBotGuildIDSet(ctx)
		if resolveErr != nil {
			if !errors.Is(resolveErr, errBotGuildIDsProviderUnavailable) {
				return nil, fmt.Errorf("accessibleGuildResolver.resolveWithOptions: %w", resolveErr)
			}
			botGuildSet = map[string]struct{}{}
		}

		freshSession, refreshErr := provider.ensureFreshSessionAccessToken(ctx, session)
		if refreshErr != nil {
			return nil, fmt.Errorf("resolve accessible guilds: refresh oauth access token: %w", refreshErr)
		}

		userGuilds, fetchErr := provider.fetchUserGuilds(ctx, freshSession.AccessToken, freshSession.TokenType)
		if fetchErr != nil {
			return nil, fmt.Errorf("resolve accessible guilds: fetch user guilds: %w", fetchErr)
		}

		cachedGuilds := make([]cachedAccessibleGuild, 0, len(userGuilds))
		for _, guild := range userGuilds {
			guildID := strings.TrimSpace(guild.ID)
			if guildID == "" {
				continue
			}
			_, botPresent := botGuildSet[guildID]
			cachedGuilds = append(cachedGuilds, cachedAccessibleGuild{
				guild:      guild,
				botPresent: botPresent,
			})
		}

		if (useCache || storeCache) && resolver.cache != nil {
			resolver.cache.Put(session, cachedGuilds)
		}
		return cachedGuilds, nil
	})

	if err != nil {
		return nil, err
	}

	cachedGuilds := v.([]cachedAccessibleGuild)
	return resolver.materializeAccessibleGuilds(cachedGuilds, session.User.ID), nil
}

func (resolver *accessibleGuildResolver) materializeAccessibleGuilds(
	guilds []cachedAccessibleGuild,
	userID string,
) []accessibleGuildResponse {
	if len(guilds) == 0 {
		return nil
	}

	out := make([]accessibleGuildResponse, 0, len(guilds))
	for _, cached := range guilds {
		accessLevel, ok := resolver.resolveGuildAccessLevel(cached.guild, userID)
		if !ok {
			continue
		}

		icon := strings.TrimSpace(cached.guild.Icon)
		if !cached.botPresent {
			icon = ""
		}

		out = append(out, accessibleGuildResponse{
			ID:          strings.TrimSpace(cached.guild.ID),
			Name:        strings.TrimSpace(cached.guild.Name),
			Icon:        icon,
			BotPresent:  cached.botPresent,
			Permissions: cached.guild.Permissions,
			AccessLevel: accessLevel,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		li := strings.ToLower(strings.TrimSpace(out[i].Name))
		lj := strings.ToLower(strings.TrimSpace(out[j].Name))
		if li == lj {
			return out[i].ID < out[j].ID
		}
		return li < lj
	})

	return out
}

func (resolver *accessibleGuildResolver) resolveBotGuildIDSet(ctx context.Context) (map[string]struct{}, error) {
	if resolver == nil || resolver.bindings == nil {
		return nil, errBotGuildIDsProviderUnavailable
	}
	return resolver.bindings.GuildIDSet(ctx)
}

func (resolver *accessibleGuildResolver) resolveGuildAccessLevel(
	guild discordOAuthGuild,
	userID string,
) (guildAccessLevel, bool) {
	if resolver == nil || resolver.evaluator == nil {
		return "", false
	}
	return resolver.evaluator.ResolveGuildAccessLevel(guild, userID)
}

func (resolver *accessibleGuildResolver) provider() *discordOAuthProvider {
	if resolver == nil || resolver.providerSource == nil {
		return nil
	}
	return resolver.providerSource()
}
