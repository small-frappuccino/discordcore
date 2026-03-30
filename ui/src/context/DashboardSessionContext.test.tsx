import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  DashboardSessionProvider,
  useDashboardSession,
} from "./DashboardSessionContext";

const mockClient = {
  getDiscordOAuthStatus: vi.fn(),
  getSessionStatus: vi.fn(),
  listAccessibleGuilds: vi.fn(),
  logout: vi.fn(),
};

vi.mock("../features/features/guildResourceCache", () => ({
  prefetchGuildDashboardResources: vi.fn().mockResolvedValue(undefined),
  resetGuildResourceCache: vi.fn(),
}));

vi.mock("../api/control", async () => {
  const actual = await vi.importActual("../api/control");

  class MockControlApiClient {
    getDiscordOAuthStatus = mockClient.getDiscordOAuthStatus;
    getSessionStatus = mockClient.getSessionStatus;
    listAccessibleGuilds = mockClient.listAccessibleGuilds;
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
  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
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

    render(
      <DashboardSessionProvider>
        <SessionProbe />
      </DashboardSessionProvider>,
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

    expect(screen.getByTestId("selected-guild")).toHaveTextContent("guild-1");
    expect(screen.getByTestId("session-loading")).toHaveTextContent("idle");
  });
});
