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
});
