import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import type {
  FeatureCatalogEntry,
  FeatureRecord,
  GuildChannelOption,
  GuildMemberOption,
  FeatureWorkspaceResponse,
  GuildRoleOption,
} from "../../api/control";
import { useFeatureCatalog } from "./useFeatureCatalog";
import { useFeatureMutation } from "./useFeatureMutation";
import { useGuildChannelOptions } from "./useGuildChannelOptions";
import { useGuildMemberOptions } from "./useGuildMemberOptions";
import { useGuildRoleOptions } from "./useGuildRoleOptions";
import { useFeatureWorkspace } from "./useFeatureWorkspace";

let mockDashboardSession: {
  authState: string;
      client: {
        getFeatureCatalog: ReturnType<typeof vi.fn>;
        listGlobalFeatures: ReturnType<typeof vi.fn>;
        listGuildChannelOptions: ReturnType<typeof vi.fn>;
        listGuildFeatures: ReturnType<typeof vi.fn>;
        listGuildMemberOptions: ReturnType<typeof vi.fn>;
        listGuildRoleOptions: ReturnType<typeof vi.fn>;
        patchGlobalFeature: ReturnType<typeof vi.fn>;
        patchGuildFeature: ReturnType<typeof vi.fn>;
    };
  selectedGuildID: string;
};

vi.mock("../../context/DashboardSessionContext", () => ({
  useDashboardSession: () => mockDashboardSession,
}));

function buildFeatureRecord(
  overrides: Partial<FeatureRecord> = {},
): FeatureRecord {
  return {
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
    ...overrides,
  };
}

function buildWorkspaceResponse(
  features: FeatureRecord[],
): FeatureWorkspaceResponse {
  return {
    status: "ok",
    workspace: {
      scope: "guild",
      guild_id: "guild-1",
      features,
    },
  };
}

describe("feature hooks", () => {
  beforeEach(() => {
    mockDashboardSession = {
      authState: "signed_in",
      client: {
        getFeatureCatalog: vi.fn(),
        listGlobalFeatures: vi.fn(),
        listGuildChannelOptions: vi.fn(),
        listGuildFeatures: vi.fn(),
        listGuildMemberOptions: vi.fn(),
        listGuildRoleOptions: vi.fn(),
        patchGlobalFeature: vi.fn(),
        patchGuildFeature: vi.fn(),
      },
      selectedGuildID: "guild-1",
    };
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("loads the feature catalog when signed in", async () => {
    const catalog: FeatureCatalogEntry[] = [
      {
        id: "logging.member_join",
        category: "logging",
        label: "Member join logging",
        description: "Record joins",
        supports_guild_override: true,
      },
    ];
    mockDashboardSession.client.getFeatureCatalog.mockResolvedValue({
      status: "ok",
      catalog,
    });

    const { result } = renderHook(() => useFeatureCatalog());

    await waitFor(() => {
      expect(result.current.catalog).toEqual(catalog);
    });

    expect(mockDashboardSession.client.getFeatureCatalog).toHaveBeenCalledTimes(1);
    expect(result.current.notice).toBeNull();
  });

  it("returns server_required for guild workspace without a selected server", () => {
    mockDashboardSession.selectedGuildID = "";

    const { result } = renderHook(() =>
      useFeatureWorkspace({
        scope: "guild",
      }),
    );

    expect(result.current.workspaceState).toBe("server_required");
    expect(mockDashboardSession.client.listGuildFeatures).not.toHaveBeenCalled();
  });

  it("loads and groups guild feature records", async () => {
    const features = [
      buildFeatureRecord({
        id: "services.commands",
        category: "services",
        label: "Commands",
      }),
      buildFeatureRecord({
        id: "logging.member_join",
        category: "logging",
        label: "Member join logging",
      }),
      buildFeatureRecord({
        id: "logging.member_leave",
        category: "logging",
        label: "Member leave logging",
      }),
    ];
    mockDashboardSession.client.listGuildFeatures.mockResolvedValue(
      buildWorkspaceResponse(features),
    );

    const { result } = renderHook(() =>
      useFeatureWorkspace({
        scope: "guild",
      }),
    );

    await waitFor(() => {
      expect(result.current.workspaceState).toBe("ready");
    });

    expect(result.current.features).toHaveLength(3);
    expect(result.current.groupedFeatures).toEqual([
      {
        category: "logging",
        features: [
          expect.objectContaining({ id: "logging.member_join" }),
          expect.objectContaining({ id: "logging.member_leave" }),
        ],
      },
      {
        category: "services",
        features: [expect.objectContaining({ id: "services.commands" })],
      },
    ]);
  });

  it("loads guild role options for the selected server", async () => {
    const roles: GuildRoleOption[] = [
      {
        id: "role-a",
        name: "Alpha",
        position: 2,
        managed: false,
        is_default: false,
      },
      {
        id: "guild-1",
        name: "@everyone",
        position: 0,
        managed: false,
        is_default: true,
      },
    ];
    mockDashboardSession.client.listGuildRoleOptions.mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      roles,
    });

    const { result } = renderHook(() => useGuildRoleOptions());

    await waitFor(() => {
      expect(result.current.roles).toEqual(roles);
    });

    expect(mockDashboardSession.client.listGuildRoleOptions).toHaveBeenCalledWith(
      "guild-1",
    );
    expect(result.current.notice).toBeNull();
  });

  it("loads guild channel options for the selected server", async () => {
    const channels: GuildChannelOption[] = [
      {
        id: "channel-a",
        name: "bot-commands",
        display_name: "#bot-commands",
        kind: "text",
        supports_message_route: true,
      },
      {
        id: "channel-v",
        name: "Voice",
        display_name: "Voice",
        kind: "voice",
        supports_message_route: false,
      },
    ];
    mockDashboardSession.client.listGuildChannelOptions.mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      channels,
    });

    const { result } = renderHook(() => useGuildChannelOptions());

    await waitFor(() => {
      expect(result.current.channels).toEqual(channels);
    });

    expect(
      mockDashboardSession.client.listGuildChannelOptions,
    ).toHaveBeenCalledWith("guild-1");
    expect(result.current.notice).toBeNull();
  });

  it("loads guild member options for the selected server", async () => {
    const members: GuildMemberOption[] = [
      {
        id: "user-1",
        display_name: "Alice",
        username: "alice",
        bot: false,
      },
    ];
    mockDashboardSession.client.listGuildMemberOptions.mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      members,
    });

    const { result } = renderHook(() =>
      useGuildMemberOptions({
        enabled: true,
        query: "ali",
        selectedMemberId: "user-2",
      }),
    );

    await waitFor(() => {
      expect(result.current.members).toEqual(members);
    });

    expect(mockDashboardSession.client.listGuildMemberOptions).toHaveBeenCalledWith(
      "guild-1",
      {
        query: "ali",
        selectedId: "user-2",
        limit: 25,
      },
    );
    expect(result.current.notice).toBeNull();
  });

  it("patches a guild feature and supports enabled: null", async () => {
    const updatedFeature = buildFeatureRecord({
      effective_enabled: false,
      override_state: "inherit",
      effective_source: "global",
    });
    mockDashboardSession.client.patchGuildFeature.mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      feature: updatedFeature,
    });

    const onSuccess = vi.fn();
    const { result } = renderHook(() =>
      useFeatureMutation({
        scope: "guild",
        onSuccess,
      }),
    );

    let response: FeatureRecord | null = null;
    await act(async () => {
      response = await result.current.patchFeature("logging.member_join", {
        enabled: null,
        channel_id: "",
      });
    });

    expect(mockDashboardSession.client.patchGuildFeature).toHaveBeenCalledWith(
      "guild-1",
      "logging.member_join",
      {
        enabled: null,
        channel_id: "",
      },
    );
    expect(response).toEqual(updatedFeature);
    expect(onSuccess).toHaveBeenCalledWith(updatedFeature);
    expect(result.current.notice).toBeNull();
  });
});
