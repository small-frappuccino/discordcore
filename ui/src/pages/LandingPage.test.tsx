import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AccessibleGuild, AuthSessionResponse } from "../api/control";
import { appRoutes } from "../app/routes";
import { LandingPage } from "./LandingPage";

const dashboardSessionMock: {
  accessibleGuilds: AccessibleGuild[];
  authState: string;
  beginLogin: ReturnType<typeof vi.fn>;
  busyLabel: string;
  manageableGuilds: AccessibleGuild[];
  notice: { message: string; tone: "info" | "error" | "success" } | null;
  logout: ReturnType<typeof vi.fn>;
  selectedGuildID: string;
  session: AuthSessionResponse | null;
  sessionAvatarURL: string | null;
  sessionLoading: boolean;
} = {
  accessibleGuilds: [],
  authState: "signed_out",
  beginLogin: vi.fn(),
  busyLabel: "",
  manageableGuilds: [],
  notice: null,
  logout: vi.fn(async () => {}),
  selectedGuildID: "",
  session: null,
  sessionAvatarURL: null,
  sessionLoading: false,
};

vi.mock("../context/DashboardSessionContext", () => ({
  useDashboardSession: () => dashboardSessionMock,
}));

describe("LandingPage", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    dashboardSessionMock.accessibleGuilds = [];
    dashboardSessionMock.authState = "signed_out";
    dashboardSessionMock.beginLogin = vi.fn();
    dashboardSessionMock.busyLabel = "";
    dashboardSessionMock.manageableGuilds = [];
    dashboardSessionMock.notice = null;
    dashboardSessionMock.logout = vi.fn(async () => {});
    dashboardSessionMock.selectedGuildID = "";
    dashboardSessionMock.session = null;
    dashboardSessionMock.sessionAvatarURL = null;
    dashboardSessionMock.sessionLoading = false;
  });

  it("uses the landing page as the Discord login return target", async () => {
    renderLandingPage();

    await userEvent.click(
      screen.getByRole("button", { name: "Login with Discord" }),
    );

    expect(dashboardSessionMock.beginLogin).toHaveBeenCalledWith(appRoutes.landing);
  });

  it("shows Control Panel and Logout after sign in without the login button", async () => {
    dashboardSessionMock.authState = "signed_in";
    dashboardSessionMock.accessibleGuilds = [
      createGuild({
        id: "guild-1",
        name: "Alpha",
      }),
    ];
    dashboardSessionMock.manageableGuilds = [
      createGuild({
        id: "guild-1",
        name: "Alpha",
      }),
    ];
    dashboardSessionMock.session = {
      status: "ok",
      user: {
        id: "user-1",
        username: "alice",
        global_name: "Alice",
      },
      scopes: ["identify"],
      csrf_token: "csrf-token",
      expires_at: "2099-01-01T00:00:00Z",
    };

    renderLandingPage();

    expect(
      screen.queryByRole("button", { name: "Login with Discord" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Logout" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Control Panel" })).toHaveAttribute(
      "href",
      appRoutes.dashboardHome("guild-1"),
    );
  });

  it("keeps Logout bound to the landing page session action", async () => {
    dashboardSessionMock.authState = "signed_in";
    dashboardSessionMock.manageableGuilds = [
      createGuild({
        id: "guild-9",
        name: "Operations",
      }),
    ];
    dashboardSessionMock.session = {
      status: "ok",
      user: {
        id: "user-1",
        username: "alice",
      },
      scopes: ["identify"],
      csrf_token: "csrf-token",
      expires_at: "2099-01-01T00:00:00Z",
    };

    renderLandingPage();

    await userEvent.click(screen.getByRole("button", { name: "Logout" }));

    expect(dashboardSessionMock.logout).toHaveBeenCalledTimes(1);
  });
});

function createGuild(overrides: Partial<AccessibleGuild>): AccessibleGuild {
  return {
    id: "guild-1",
    name: "Test Guild",
    owner: true,
    permissions: 0,
    access_level: "write",
    ...overrides,
  };
}

function renderLandingPage() {
  return render(
    <MemoryRouter>
      <LandingPage />
    </MemoryRouter>,
  );
}
