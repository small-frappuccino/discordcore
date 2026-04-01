import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import App from "./App";
import { appRoutes } from "./app/routes";

const testGuildID = "guild-1";
const testRoutes = {
  coreCommands: appRoutes.dashboardCoreCommands(testGuildID),
};

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });

  return {
    promise,
    reject,
    resolve,
  };
}

describe("dashboard session hydration", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("keeps the routed dashboard content gated until auth, guild access, and the first guild workspace are ready", async () => {
    const authDeferred = createDeferred<unknown>();
    const guildsDeferred = createDeferred<unknown>();
    const featuresDeferred = createDeferred<unknown>();
    const rolesDeferred = createDeferred<unknown>();
    const channelsDeferred = createDeferred<unknown>();

    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;

      if (url.endsWith("/auth/me")) {
        return authDeferred.promise.then((body) => jsonResponse(body));
      }

      if (url.endsWith("/auth/guilds/access")) {
        return guildsDeferred.promise.then((body) => jsonResponse(body));
      }

      if (url.endsWith("/auth/guilds/manageable")) {
        return guildsDeferred.promise.then((body) => jsonResponse(body));
      }

      if (url.endsWith("/v1/guilds/guild-1/features")) {
        return featuresDeferred.promise.then((body) => jsonResponse(body));
      }

      if (url.endsWith("/v1/guilds/guild-1/role-options")) {
        return rolesDeferred.promise.then((body) => jsonResponse(body));
      }

      if (url.endsWith("/v1/guilds/guild-1/channel-options")) {
        return channelsDeferred.promise.then((body) => jsonResponse(body));
      }

      throw new Error(`Unexpected fetch: ${url}`);
    });

    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreCommands);

    render(<App />);

    expect(
      screen.getByRole("heading", { name: "Loading dashboard", level: 2 }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Commands", level: 1 }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Sign in required")).not.toBeInTheDocument();

    authDeferred.resolve({
      status: "ok",
      user: {
        id: "user-1",
        username: "alice",
        global_name: "Alice",
      },
      scopes: ["identify", "guilds"],
      csrf_token: "csrf-token",
      expires_at: "2099-01-01T00:00:00Z",
    });

    await waitFor(() => {
      expect(
        fetchMock.mock.calls.some(([url]) =>
          String(url).includes("/auth/guilds/access"),
        ),
      ).toBe(true);
    });

    guildsDeferred.resolve({
      status: "ok",
      count: 1,
      guilds: [
        {
          id: "guild-1",
          name: "Server One",
          owner: true,
          permissions: 8,
          access_level: "write",
        },
      ],
    });

    await waitFor(() => {
      expect(
        fetchMock.mock.calls.some(([url]) =>
          String(url).includes("/v1/guilds/guild-1/features"),
        ),
      ).toBe(true);
      expect(
        fetchMock.mock.calls.some(([url]) =>
          String(url).includes("/v1/guilds/guild-1/role-options"),
        ),
      ).toBe(true);
      expect(
        fetchMock.mock.calls.some(([url]) =>
          String(url).includes("/v1/guilds/guild-1/channel-options"),
        ),
      ).toBe(true);
    });

    expect(
      screen.getByRole("heading", { name: "Loading dashboard", level: 2 }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Commands", level: 1 }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Sign in required")).not.toBeInTheDocument();

    featuresDeferred.resolve({
      status: "ok",
      workspace: {
        scope: "guild",
        guild_id: "guild-1",
        features: [
          {
            id: "services.commands",
            category: "services",
            label: "Commands",
            description: "Commands",
            scope: "guild",
            supports_guild_override: true,
            override_state: "enabled",
            effective_enabled: true,
            effective_source: "guild",
            readiness: "ready",
            details: {
              channel_id: "bot-commands",
            },
            editable_fields: ["enabled", "channel_id"],
          },
          {
            id: "services.admin_commands",
            category: "services",
            label: "Admin commands",
            description: "Admin commands",
            scope: "guild",
            supports_guild_override: true,
            override_state: "enabled",
            effective_enabled: true,
            effective_source: "guild",
            readiness: "ready",
            details: {
              allowed_role_ids: ["role-guard"],
              allowed_role_count: 1,
            },
            editable_fields: ["enabled", "allowed_role_ids"],
          },
        ],
      },
    });
    rolesDeferred.resolve({
      status: "ok",
      guild_id: "guild-1",
      roles: [
        {
          id: "guild-1",
          name: "@everyone",
          position: 0,
          managed: false,
          is_default: true,
        },
        {
          id: "role-guard",
          name: "Moderators",
          position: 1,
          managed: false,
          is_default: false,
        },
      ],
    });
    channelsDeferred.resolve({
      status: "ok",
      guild_id: "guild-1",
      channels: [
        {
          id: "bot-commands",
          name: "bot-commands",
          display_name: "#bot-commands",
          kind: "text",
          supports_message_route: true,
        },
      ],
    });

    await screen.findByRole("heading", { name: "Commands", level: 1 });

    expect(
      screen.queryByRole("heading", { name: "Loading dashboard", level: 2 }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Command routing", level: 2 }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Sign in required")).not.toBeInTheDocument();
  });
});
