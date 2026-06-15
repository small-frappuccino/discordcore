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
  listAccessibleGuildsMock,
  listManageableGuildsMock,
} = vi.hoisted(() => ({
  prefetchGuildDashboardResourcesMock: vi.fn().mockResolvedValue(undefined),
  resetGuildResourceCacheMock: vi.fn(),
  listAccessibleGuildsMock: vi.fn(),
  listManageableGuildsMock: vi.fn(),
}));

const mockClient = {
  getDiscordOAuthStatus: vi.fn(),
  getSessionStatus: vi.fn(),
  logout: vi.fn(),
};

vi.mock("../features/features/guildResourceCache", () => ({
  prefetchGuildDashboardResources: prefetchGuildDashboardResourcesMock,
  resetGuildResourceCache: resetGuildResourceCacheMock,
}));

vi.mock("../api/domains/guilds", () => ({
  listAccessibleGuilds: listAccessibleGuildsMock,
  listManageableGuilds: listManageableGuildsMock,
}));

vi.mock("../api/client", async () => {
  const actual = await vi.importActual("../api/client");

  class MockControlApiClient {
    getDiscordOAuthStatus = mockClient.getDiscordOAuthStatus;
    getSessionStatus = mockClient.getSessionStatus;
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
  const { authState, sessionLoading } = useDashboardSession();

  return (
    <div>
      <dl>
        <dt>Auth State</dt>
        <dd data-testid="auth-state">{authState}</dd>
        <dt>Session Loading</dt>
        <dd data-testid="session-loading">{String(sessionLoading)}</dd>
      </dl>
    </div>
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
        permissions: number;
        access_level: "write";
      }>;
    }>();

    mockClient.getSessionStatus.mockReturnValue(sessionDeferred.promise);
    listAccessibleGuildsMock.mockReturnValue(guildsDeferred.promise);
    listManageableGuildsMock.mockResolvedValue({
      status: "ok",
      count: 1,
      guilds: [
        {
          id: "guild-1",
          name: "Server One",
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
    expect(screen.getByTestId("session-loading")).toHaveTextContent("true");

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
      expect(listAccessibleGuildsMock).toHaveBeenCalled();
    });

    expect(screen.getByTestId("auth-state")).toHaveTextContent("checking");

    guildsDeferred.resolve({
      status: "ok",
      count: 1,
      guilds: [
        {
          id: "guild-1",
          name: "Server One",
          permissions: 8,
          access_level: "write",
        },
      ],
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-state")).toHaveTextContent("signed_in");
    });

    expect(screen.getByTestId("session-loading")).toHaveTextContent("false");
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
    listAccessibleGuildsMock
      .mockResolvedValueOnce({
        status: "ok",
        count: 1,
        guilds: [
          {
            id: "guild-1",
            name: "Server One",
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
    listManageableGuildsMock
      .mockResolvedValueOnce({
        status: "ok",
        count: 1,
        guilds: [
          {
            id: "guild-1",
            name: "Server One",
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
    expect(listAccessibleGuildsMock).toHaveBeenNthCalledWith(1, expect.any(Object), {
      fresh: true,
    });

    now = new Date("2026-04-01T00:00:20Z").getTime();

    await act(async () => {
      window.dispatchEvent(new Event("focus"));
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(listAccessibleGuildsMock).toHaveBeenNthCalledWith(2, expect.any(Object), {
        fresh: true,
      });
    });
  });
});
