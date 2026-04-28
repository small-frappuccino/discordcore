import { afterEach, describe, expect, it, vi } from "vitest";
import { ControlApiClient, type FeatureRecord } from "./control";

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

describe("ControlApiClient feature routes", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("loads the feature catalog from /v1/features/catalog", async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        status: "ok",
        catalog: [
          {
            id: "logging.member_join",
            category: "logging",
            label: "Member join logging",
            description: "Record joins",
            supports_guild_override: true,
          },
        ],
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ControlApiClient({
      baseUrl: "",
    });

    const response = await client.getFeatureCatalog();

    expect(response.catalog).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledWith("/v1/features/catalog", {
      method: "GET",
      headers: expect.any(Headers),
      credentials: "include",
      body: undefined,
    });
  });

  it("patches a guild feature with PATCH and a CSRF token", async () => {
    const updatedFeature: FeatureRecord = {
      id: "logging.member_join",
      category: "logging",
      label: "Member join logging",
      description: "Record joins",
      scope: "guild",
      supports_guild_override: true,
      override_state: "enabled",
      effective_enabled: true,
      effective_source: "guild",
      readiness: "ready",
      blockers: [],
      details: {},
      editable_fields: ["enabled", "channel_id"],
    };
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        void init;
        const url = typeof input === "string" ? input : input.toString();

        if (url.endsWith("/auth/me")) {
          return jsonResponse({
            status: "ok",
            user: {
              id: "user-1",
              username: "alice",
            },
            scopes: ["identify"],
            csrf_token: "csrf-token",
            expires_at: "2099-01-01T00:00:00Z",
          });
        }

        return jsonResponse({
          status: "ok",
          guild_id: "guild-1",
          feature: updatedFeature,
        });
      },
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ControlApiClient({
      baseUrl: "",
    });

    const response = await client.patchGuildFeature("guild-1", "logging.member_join", {
      enabled: null,
      channel_id: "",
    });

    expect(response.feature).toEqual(updatedFeature);
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/v1/guilds/guild-1/features/logging.member_join",
      {
        method: "PATCH",
        headers: expect.objectContaining({
          get: expect.any(Function),
        }),
        credentials: "include",
        body: JSON.stringify({
          enabled: null,
          channel_id: "",
        }),
      },
    );

    const secondCall = fetchMock.mock.calls[1];
    const headers = secondCall?.[1]?.headers as Headers;
    expect(headers.get("X-CSRF-Token")).toBe("csrf-token");
  });

  it("patches the commands channel with the expected payload", async () => {
    const updatedFeature: FeatureRecord = {
      id: "services.commands",
      category: "services",
      label: "Commands",
      description: "Route commands for this server.",
      scope: "guild",
      supports_guild_override: true,
      override_state: "enabled",
      effective_enabled: true,
      effective_source: "guild",
      readiness: "ready",
      blockers: [],
      details: {
        channel_id: "bot-commands",
      },
      editable_fields: ["enabled", "channel_id"],
    };
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        void init;
        const url = typeof input === "string" ? input : input.toString();

        if (url.endsWith("/auth/me")) {
          return jsonResponse({
            status: "ok",
            user: {
              id: "user-1",
              username: "alice",
            },
            scopes: ["identify"],
            csrf_token: "csrf-token",
            expires_at: "2099-01-01T00:00:00Z",
          });
        }

        return jsonResponse({
          status: "ok",
          guild_id: "guild-1",
          feature: updatedFeature,
        });
      },
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ControlApiClient({
      baseUrl: "",
    });

    const response = await client.patchGuildFeature("guild-1", "services.commands", {
      channel_id: "bot-commands",
    });

    expect(response.feature).toEqual(updatedFeature);
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/v1/guilds/guild-1/features/services.commands",
      {
        method: "PATCH",
        headers: expect.objectContaining({
          get: expect.any(Function),
        }),
        credentials: "include",
        body: JSON.stringify({
          channel_id: "bot-commands",
        }),
      },
    );
  });

  it("patches admin command roles with the expected payload", async () => {
    const updatedFeature: FeatureRecord = {
      id: "services.admin_commands",
      category: "services",
      label: "Admin commands",
      description: "Privileged command access for this server.",
      scope: "guild",
      supports_guild_override: true,
      override_state: "enabled",
      effective_enabled: true,
      effective_source: "guild",
      readiness: "ready",
      blockers: [],
      details: {
        allowed_role_ids: ["role-guard", "role-target"],
      },
      editable_fields: ["enabled", "allowed_role_ids"],
    };
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        void init;
        const url = typeof input === "string" ? input : input.toString();

        if (url.endsWith("/auth/me")) {
          return jsonResponse({
            status: "ok",
            user: {
              id: "user-1",
              username: "alice",
            },
            scopes: ["identify"],
            csrf_token: "csrf-token",
            expires_at: "2099-01-01T00:00:00Z",
          });
        }

        return jsonResponse({
          status: "ok",
          guild_id: "guild-1",
          feature: updatedFeature,
        });
      },
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ControlApiClient({
      baseUrl: "",
    });

    const response = await client.patchGuildFeature(
      "guild-1",
      "services.admin_commands",
      {
        allowed_role_ids: ["role-guard", "role-target"],
      },
    );

    expect(response.feature).toEqual(updatedFeature);
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/v1/guilds/guild-1/features/services.admin_commands",
      {
        method: "PATCH",
        headers: expect.objectContaining({
          get: expect.any(Function),
        }),
        credentials: "include",
        body: JSON.stringify({
          allowed_role_ids: ["role-guard", "role-target"],
        }),
      },
    );
  });

  it("loads qotd summary with the expected guild-scoped path", async () => {
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL) => {
        const url = typeof input === "string" ? input : input.toString();

        if (url.endsWith("/auth/me")) {
          return jsonResponse({
            status: "ok",
            user: {
              id: "user-1",
              username: "alice",
            },
            scopes: ["identify"],
            csrf_token: "csrf-token",
            expires_at: "2099-01-01T00:00:00Z",
          });
        }

        if (url.endsWith("/qotd")) {
          return jsonResponse({
            status: "ok",
            guild_id: "guild-1",
            summary: {
              settings: {
                active_deck_id: "default",
                decks: [
                  {
                    id: "default",
                    name: "Default",
                    enabled: true,
                    channel_id: "question-channel-1",
                  },
                ],
              },
              counts: {
                total: 1,
                draft: 0,
                ready: 1,
                reserved: 0,
                used: 0,
                disabled: 0,
              },
              decks: [
                {
                  id: "default",
                  name: "Default",
                  enabled: true,
                  counts: {
                    total: 1,
                    draft: 0,
                    ready: 1,
                    reserved: 0,
                    used: 0,
                    disabled: 0,
                  },
                  cards_remaining: 1,
                  is_active: true,
                  can_publish: true,
                },
              ],
              current_publish_date_utc: "2026-04-03T00:00:00Z",
              published_for_current_slot: true,
              current_post: {
                deck_id: "default",
                deck_name: "Default",
                publish_mode: "scheduled",
                publish_date_utc: "2026-04-03T00:00:00Z",
                state: "current",
                question_text: "What should we build next?",
                published_at: "2026-04-03T00:00:00Z",
                becomes_previous_at: "2026-04-04T00:00:00Z",
                answers_close_at: "2026-04-05T00:00:00Z",
                thread_id: "thread-20260403",
                thread_url: "https://discord.com/channels/guild-1/thread-20260403",
                answer_channel_id: "thread-20260403",
                answer_channel_url:
                  "https://discord.com/channels/guild-1/thread-20260403",
                post_url:
                  "https://discord.com/channels/guild-1/question-channel-1/message-20260403",
              },
              previous_post: {
                deck_id: "default",
                deck_name: "Default",
                publish_mode: "scheduled",
                publish_date_utc: "2026-04-02T00:00:00Z",
                state: "previous",
                question_text: "What did we ship yesterday?",
                published_at: "2026-04-02T00:00:00Z",
                becomes_previous_at: "2026-04-03T00:00:00Z",
                answers_close_at: "2026-04-04T00:00:00Z",
                thread_id: "thread-20260402",
                thread_url: "https://discord.com/channels/guild-1/thread-20260402",
                answer_channel_id: "thread-20260402",
                answer_channel_url:
                  "https://discord.com/channels/guild-1/thread-20260402",
                post_url:
                  "https://discord.com/channels/guild-1/question-channel-1/message-20260402",
              },
            },
          });
        }

        throw new Error(`Unexpected request: ${url}`);
      },
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ControlApiClient({
      baseUrl: "",
    });

    const summary = await client.getQOTDSummary("guild-1");

    expect(summary.summary.settings.active_deck_id).toBe("default");
    expect(summary.summary.published_for_current_slot).toBe(true);
    expect(summary.summary.current_post?.state).toBe("current");
    expect(summary.summary.previous_post?.state).toBe("previous");
    expect(summary.summary.current_post?.thread_id).toBe("thread-20260403");
    expect(summary.summary.current_post?.answer_channel_url).toBe(
      "https://discord.com/channels/guild-1/thread-20260403",
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/v1/guilds/guild-1/qotd",
      {
        method: "GET",
        headers: expect.any(Headers),
        credentials: "include",
        body: undefined,
      },
    );
  });

  it("loads guild role options from /v1/guilds/{guild_id}/role-options", async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        status: "ok",
        guild_id: "guild-1",
        roles: [
          {
            id: "role-1",
            name: "Moderators",
            position: 3,
            managed: false,
            is_default: false,
          },
        ],
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ControlApiClient({
      baseUrl: "",
    });

    const response = await client.listGuildRoleOptions("guild-1");

    expect(response.roles).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledWith("/v1/guilds/guild-1/role-options", {
      method: "GET",
      headers: expect.any(Headers),
      credentials: "include",
      body: undefined,
    });
  });

  it("loads guild channel options from /v1/guilds/{guild_id}/channel-options", async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        status: "ok",
        guild_id: "guild-1",
        channels: [
          {
            id: "channel-1",
            name: "bot-commands",
            display_name: "#bot-commands",
            kind: "text",
            supports_message_route: true,
          },
        ],
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ControlApiClient({
      baseUrl: "",
    });

    const response = await client.listGuildChannelOptions("guild-1");

    expect(response.channels).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledWith("/v1/guilds/guild-1/channel-options", {
      method: "GET",
      headers: expect.any(Headers),
      credentials: "include",
      body: undefined,
    });
  });

  it("retries transient GET failures before surfacing an error", async () => {
    vi.useFakeTimers();

    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response("temporary failure", {
          status: 502,
          statusText: "Bad Gateway",
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          status: "ok",
          guild_id: "guild-1",
          channels: [
            {
              id: "channel-1",
              name: "bot-commands",
              display_name: "#bot-commands",
              kind: "text",
              supports_message_route: true,
            },
          ],
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ControlApiClient({
      baseUrl: "",
    });

    const responsePromise = client.listGuildChannelOptions("guild-1");
    await vi.advanceTimersByTimeAsync(80);
    const response = await responsePromise;

    expect(response.channels).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledTimes(2);

  });

  it("loads guild member options with query parameters", async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        status: "ok",
        guild_id: "guild-1",
        members: [
          {
            id: "user-1",
            display_name: "Alice",
            username: "alice",
            bot: false,
          },
        ],
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ControlApiClient({
      baseUrl: "",
    });

    const response = await client.listGuildMemberOptions("guild-1", {
      query: "ali",
      selectedId: "user-2",
      limit: 10,
    });

    expect(response.members).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledWith(
      "/v1/guilds/guild-1/member-options?query=ali&selected_id=user-2&limit=10",
      {
        method: "GET",
        headers: expect.any(Headers),
        credentials: "include",
        body: undefined,
      },
    );
  });

  it("preserves the landing page as the OAuth return target when requested", async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        status: "ok",
        oauth_configured: true,
        authenticated: false,
        dashboard_url: "/manage/",
        login_url: "/auth/discord/login?next=%2F",
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ControlApiClient({
      baseUrl: "",
    });

    const response = await client.getDiscordOAuthStatus("/");

    expect(response.login_url).toBe("/auth/discord/login?next=%2F");
    expect(fetchMock).toHaveBeenCalledWith("/auth/discord/status?next=%2F", {
      method: "GET",
      credentials: "include",
    });
  });
});
