import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  AccessibleGuild,
  FeatureRecord,
} from "../api/control";
import { HomePage } from "./HomePage";

const dashboardSessionMock: {
  authState: string;
  beginLogin: ReturnType<typeof vi.fn>;
  currentOriginLabel: string;
  selectedGuild: AccessibleGuild | null;
  selectedGuildID: string;
} = {
  authState: "checking",
  beginLogin: vi.fn(),
  currentOriginLabel: "Local control server",
  selectedGuild: null,
  selectedGuildID: "guild-1",
};

const featureWorkspaceMock: {
  features: FeatureRecord[];
  groupedFeatures: unknown[];
  loading: boolean;
  notice: null;
  scope: string;
  workspace: null;
  workspaceState: string;
  clearNotice: () => void;
  refresh: () => Promise<void>;
} = {
  features: [] as FeatureRecord[],
  groupedFeatures: [],
  loading: false,
  notice: null,
  scope: "guild",
  workspace: null,
  workspaceState: "checking" as const,
  clearNotice: () => {},
  refresh: async () => {},
};

const rolesSettingsMock = {
  roles: {
    allowedRoleIds: [],
    dashboardReadRoleIds: [] as string[],
    dashboardWriteRoleIds: [] as string[],
  },
  loading: false,
  notice: null,
  refresh: async () => {},
  updateCachedRoles: () => {},
  clearNotice: () => {},
};

const partnerBoardMock = {
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
};

vi.mock("../context/DashboardSessionContext", () => ({
  useDashboardSession: () => dashboardSessionMock,
}));

vi.mock("../features/features/useFeatureWorkspace", () => ({
  useFeatureWorkspace: () => featureWorkspaceMock,
}));

vi.mock("../features/control-panel/useGuildRolesSettings", () => ({
  useGuildRolesSettings: () => rolesSettingsMock,
}));

vi.mock("../features/partner-board/usePartnerBoardSummary", () => ({
  usePartnerBoardSummary: () => partnerBoardMock,
}));

describe("HomePage", () => {
  it("keeps cards in loading state while the session is still checking", () => {
    renderHomePage();

    expect(screen.getByRole("heading", { name: "Home", level: 1 })).toBeInTheDocument();
    expect(screen.queryAllByText("Status: Sign in required")).toHaveLength(0);
    expect(screen.queryAllByText("Server: Select a server")).toHaveLength(0);
    expect(document.querySelectorAll('.home-nav-card[aria-busy="true"]')).not.toHaveLength(0);
  });

  it("uses Enabled and Disabled on overview cards instead of On and Off", () => {
    dashboardSessionMock.authState = "signed_in";
    dashboardSessionMock.selectedGuild = {
      id: "guild-1",
      name: "Test Guild",
      icon: undefined,
      owner: true,
      permissions: 0,
      access_level: "write",
    };
    dashboardSessionMock.selectedGuildID = "guild-1";

    featureWorkspaceMock.workspaceState = "ready";
    featureWorkspaceMock.features = [
      createFeatureRecord({
        id: "stats_channels",
        label: "Stats",
        effective_enabled: true,
        details: {
          configured_channel_count: 2,
          update_interval_mins: 30,
        },
      }),
      createFeatureRecord({
        id: "services.commands",
        label: "Commands",
        effective_enabled: false,
        details: {
          channel_id: "",
        },
      }),
      createFeatureRecord({
        id: "services.admin_commands",
        label: "Admin commands",
        effective_enabled: true,
        details: {
          allowed_role_count: 2,
        },
      }),
    ];

    renderHomePage();

    expect(screen.getByText("Enabled")).toBeInTheDocument();
    expect(screen.getByText("Disabled")).toBeInTheDocument();
    expect(screen.queryByText("On")).not.toBeInTheDocument();
    expect(screen.queryByText("Off")).not.toBeInTheDocument();
  });
});

beforeEach(() => {
  dashboardSessionMock.authState = "checking";
  dashboardSessionMock.currentOriginLabel = "Local control server";
  dashboardSessionMock.selectedGuild = null;
  dashboardSessionMock.selectedGuildID = "guild-1";

  featureWorkspaceMock.features = [];
  featureWorkspaceMock.loading = false;
  featureWorkspaceMock.notice = null;
  featureWorkspaceMock.scope = "guild";
  featureWorkspaceMock.workspace = null;
  featureWorkspaceMock.workspaceState = "checking";

  rolesSettingsMock.roles.dashboardReadRoleIds = [];
  rolesSettingsMock.roles.dashboardWriteRoleIds = [];
  rolesSettingsMock.loading = false;
  rolesSettingsMock.notice = null;

  partnerBoardMock.board = null;
  partnerBoardMock.deliveryConfigured = false;
  partnerBoardMock.layoutConfigured = false;
  partnerBoardMock.loading = false;
  partnerBoardMock.notice = null;
  partnerBoardMock.partnerCount = 0;
});

function createFeatureRecord(overrides: Partial<FeatureRecord>): FeatureRecord {
  return {
    id: "feature",
    category: "core",
    label: "Feature",
    description: "Feature description",
    scope: "guild",
    supports_guild_override: true,
    override_state: "configured",
    effective_enabled: false,
    effective_source: "guild",
    readiness: "ready",
    blockers: [],
    details: {},
    editable_fields: [],
    ...overrides,
  };
}

function renderHomePage() {
  return render(
    <MemoryRouter>
      <HomePage />
    </MemoryRouter>,
  );
}
