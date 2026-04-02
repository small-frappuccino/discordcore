import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { appRoutes } from "../app/routes";
import {
  DashboardSessionProvider,
  useDashboardSession,
} from "./DashboardSessionContext";

const {
  prefetchGuildDashboardResourcesMock,
  resetGuildResourceCacheMock,
} = vi.hoisted(() => ({
  prefetchGuildDashboardResourcesMock: vi.fn().mockResolvedValue(undefined),
  resetGuildResourceCacheMock: vi.fn(),
}));

const mockClient = {
  getDiscordOAuthStatus: vi.fn(),
  getSessionStatus: vi.fn(),
  listAccessibleGuilds: vi.fn(),
  listManageableGuilds: vi.fn(),
  logout: vi.fn(),
};

vi.mock("../features/features/guildResourceCache", () => ({
  prefetchGuildDashboardResources: prefetchGuildDashboardResourcesMock,
  resetGuildResourceCache: resetGuildResourceCacheMock,
}));

vi.mock("../api/control", async () => {
  const actual = await vi.importActual("../api/control");

  class MockControlApiClient {
    getDiscordOAuthStatus = mockClient.getDiscordOAuthStatus;
    getSessionStatus = mockClient.getSessionStatus;
    listAccessibleGuilds = mockClient.listAccessibleGuilds;
    listManageableGuilds = mockClient.listManageableGuilds;
    logout = mockClient.logout;

    constructor(config: unknown) {
      void config;
    }
  }

  return {
    ...actual,
    ControlApiClient: MockControlApiClient,
  };
});

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

function SessionProbe() {
  const { authState, selectedGuildID, sessionLoading } = useDashboardSession();

  return (
    <dl>
      <div>
        <dt>auth</dt>
        <dd data-testid="auth-state">{authState}</dd>
      </div>
      <div>
        <dt>guild</dt>
        <dd data-testid="selected-guild">{selectedGuildID || "(none)"}</dd>
      </div>
      <div>
        <dt>loading</dt>
        <dd data-testid="session-loading">{sessionLoading ? "loading" : "idle"}</dd>
      </div>
    </dl>
  );
}

describe("DashboardSessionProvider", () => {
  beforeEach(() => {
    prefetchGuildDashboardResourcesMock.mockResolvedValue(undefined);
    resetGuildResourceCacheMock.mockImplementation(() => {});
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.restoreAllMocks();
    vi.useRealTimers();
    window.history.replaceState({}, "", "/");
  });

  it("keeps authState checking until the accessible guild list resolves", async () => {
    const sessionDeferred = createDeferred<{
      status: "authenticated";
      session: {
        status: string;
        user: {
          id: string;
          username: string;
          global_name: string;
        };
        scopes: string[];
        csrf_token: string;
        expires_at: string;
      };
    }>();
    const guildsDeferred = createDeferred<{
      status: string;
      count: number;
      guilds: Array<{
        id: string;
        name: string;
        owner: boolean;
        permissions: number;
        access_level: "write";
      }>;
    }>();

    mockClient.getSessionStatus.mockReturnValue(sessionDeferred.promise);
    mockClient.listAccessibleGuilds.mockReturnValue(guildsDeferred.promise);
    mockClient.listManageableGuilds.mockResolvedValue({
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

    render(
      <MemoryRouter initialEntries={[appRoutes.dashboardHome("guild-1")]}>
        <DashboardSessionProvider>
          <SessionProbe />
        </DashboardSessionProvider>
      </MemoryRouter>,
    );

    expect(screen.getByTestId("auth-state")).toHaveTextContent("checking");
    expect(screen.getByTestId("selected-guild")).toHaveTextContent("(none)");
    expect(screen.getByTestId("session-loading")).toHaveTextContent("loading");

    sessionDeferred.resolve({
      status: "authenticated",
      session: {
        status: "ok",
        user: {
          id: "user-1",
          username: "alice",
          global_name: "alice",
        },
        scopes: ["identify", "guilds"],
        csrf_token: "csrf-token",
        expires_at: "2099-01-01T00:00:00Z",
      },
    });

    await waitFor(() => {
      expect(mockClient.listAccessibleGuilds).toHaveBeenCalled();
    });

    expect(screen.getByTestId("auth-state")).toHaveTextContent("checking");
    expect(screen.getByTestId("selected-guild")).toHaveTextContent("(none)");

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
      expect(screen.getByTestId("auth-state")).toHaveTextContent("signed_in");
    });
    expect(mockClient.listManageableGuilds).toHaveBeenCalled();

    expect(screen.getByTestId("selected-guild")).toHaveTextContent("guild-1");
    expect(screen.getByTestId("session-loading")).toHaveTextContent("idle");
  });

  it("uses the router pathname during refresh inside MemoryRouter", async () => {
    const prefetchDeferred = createDeferred<void>();

    prefetchGuildDashboardResourcesMock.mockReturnValueOnce(prefetchDeferred.promise);
    mockClient.getSessionStatus.mockResolvedValue({
      status: "authenticated",
      session: {
        status: "ok",
        user: {
          id: "user-1",
          username: "alice",
          global_name: "alice",
        },
        scopes: ["identify", "guilds"],
        csrf_token: "csrf-token",
        expires_at: "2099-01-01T00:00:00Z",
      },
    });
    mockClient.listAccessibleGuilds.mockResolvedValue({
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
    mockClient.listManageableGuilds.mockResolvedValue({
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

    window.history.replaceState({}, "", "/");

    render(
      <MemoryRouter initialEntries={[appRoutes.dashboardHome("guild-1")]}>
        <DashboardSessionProvider>
          <SessionProbe />
        </DashboardSessionProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(prefetchGuildDashboardResourcesMock).toHaveBeenCalledWith(
        expect.any(Object),
        expect.any(String),
        "guild-1",
      );
    });

    expect(screen.getByTestId("session-loading")).toHaveTextContent("loading");
    expect(screen.getByTestId("auth-state")).toHaveTextContent("checking");

    prefetchDeferred.resolve();

    await waitFor(() => {
      expect(screen.getByTestId("auth-state")).toHaveTextContent("signed_in");
    });

    expect(screen.getByTestId("selected-guild")).toHaveTextContent("guild-1");
    expect(screen.getByTestId("session-loading")).toHaveTextContent("idle");
  });

  it("revalidates guild access on window focus and clears an inaccessible routed guild", async () => {
    let now = new Date("2026-04-01T00:00:00Z").getTime();
    vi.spyOn(Date, "now").mockImplementation(() => now);

    mockClient.getSessionStatus.mockResolvedValue({
      status: "authenticated",
      session: {
        status: "ok",
        user: {
          id: "user-1",
          username: "alice",
          global_name: "alice",
        },
        scopes: ["identify", "guilds"],
        csrf_token: "csrf-token",
        expires_at: "2099-01-01T00:00:00Z",
      },
    });
    mockClient.listAccessibleGuilds
      .mockResolvedValueOnce({
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
      })
      .mockResolvedValueOnce({
        status: "ok",
        count: 0,
        guilds: [],
      });
    mockClient.listManageableGuilds
      .mockResolvedValueOnce({
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
      })
      .mockResolvedValueOnce({
        status: "ok",
        count: 0,
        guilds: [],
      });

    render(
      <MemoryRouter initialEntries={[appRoutes.dashboardHome("guild-1")]}>
        <DashboardSessionProvider>
          <SessionProbe />
        </DashboardSessionProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-state")).toHaveTextContent("signed_in");
    });
    expect(screen.getByTestId("selected-guild")).toHaveTextContent("guild-1");
    expect(mockClient.listAccessibleGuilds).toHaveBeenNthCalledWith(1, {
      fresh: true,
    });

    now = new Date("2026-04-01T00:00:20Z").getTime();

    await act(async () => {
      window.dispatchEvent(new Event("focus"));
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(mockClient.listAccessibleGuilds).toHaveBeenNthCalledWith(2, {
        fresh: true,
      });
    });
    await waitFor(() => {
      expect(screen.getByTestId("selected-guild")).toHaveTextContent("(none)");
    });
  });
});
