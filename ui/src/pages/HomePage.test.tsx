import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { HomePage } from "./HomePage";

vi.mock("../context/DashboardSessionContext", () => ({
  useDashboardSession: () => ({
    authState: "checking",
    selectedGuild: null,
  }),
}));

vi.mock("../features/features/useFeatureWorkspace", () => ({
  useFeatureWorkspace: () => ({
    features: [],
    groupedFeatures: [],
    loading: false,
    notice: null,
    scope: "guild",
    workspace: null,
    workspaceState: "checking",
    clearNotice: () => {},
    refresh: async () => {},
  }),
}));

vi.mock("../features/control-panel/useGuildRolesSettings", () => ({
  useGuildRolesSettings: () => ({
    roles: {
      allowedRoleIds: [],
      dashboardReadRoleIds: [],
      dashboardWriteRoleIds: [],
    },
    loading: false,
    notice: null,
    refresh: async () => {},
    updateCachedRoles: () => {},
    clearNotice: () => {},
  }),
}));

vi.mock("../features/partner-board/usePartnerBoardSummary", () => ({
  usePartnerBoardSummary: () => ({
    board: null,
    clearNotice: () => {},
    deliveryConfigured: false,
    hasLoadedAttempt: false,
    lastLoadedAt: null,
    layoutConfigured: false,
    loading: false,
    notice: null,
    partnerCount: 0,
    postingMethodLabel: "",
    refreshBoardSummary: async () => {},
    shellStatus: {
      tone: "info",
      label: "Loading board",
      description: "Loading the latest board settings for this server.",
    },
    summarizePostingDestination: () => "Not configured",
  }),
}));

describe("HomePage", () => {
  it("keeps cards in loading state while the session is still checking", () => {
    render(<HomePage />);

    expect(screen.getByRole("heading", { name: "Home", level: 1 })).toBeInTheDocument();
    expect(screen.queryAllByText("Status: Sign in required")).toHaveLength(0);
    expect(screen.queryAllByText("Server: Select a server")).toHaveLength(0);
    expect(document.querySelectorAll('.home-nav-card[aria-busy="true"]')).not.toHaveLength(0);
  });
});
