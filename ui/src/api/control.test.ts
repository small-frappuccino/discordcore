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
      async (input: RequestInfo | URL, _init?: RequestInit) => {
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
      async (input: RequestInfo | URL, _init?: RequestInit) => {
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
      async (input: RequestInfo | URL, _init?: RequestInit) => {
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
});
