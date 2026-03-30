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
import { resetGuildResourceCache } from "./guildResourceCache";

let mockDashboardSession: {
  authState: string;
  baseUrl: string;
  canEditSelectedGuild: boolean;
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
      baseUrl: "",
      canEditSelectedGuild: true,
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
    resetGuildResourceCache();
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

    expect(mockDashboardSession.client.getFeatureCatalog).toHaveBeenCalledTimes(
      1,
    );
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
    expect(
      mockDashboardSession.client.listGuildFeatures,
    ).not.toHaveBeenCalled();
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

  it("reuses cached guild workspace data across mounts", async () => {
    const features = [
      buildFeatureRecord({
        id: "services.commands",
        category: "services",
        label: "Commands",
      }),
    ];
    mockDashboardSession.client.listGuildFeatures.mockResolvedValue(
      buildWorkspaceResponse(features),
    );

    const firstHook = renderHook(() =>
      useFeatureWorkspace({
        scope: "guild",
      }),
    );

    await waitFor(() => {
      expect(firstHook.result.current.workspaceState).toBe("ready");
    });
    firstHook.unmount();

    const secondHook = renderHook(() =>
      useFeatureWorkspace({
        scope: "guild",
      }),
    );

    expect(secondHook.result.current.workspaceState).toBe("ready");
    expect(secondHook.result.current.features).toEqual(features);
    expect(mockDashboardSession.client.listGuildFeatures).toHaveBeenCalledTimes(1);
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

    expect(
      mockDashboardSession.client.listGuildRoleOptions,
    ).toHaveBeenCalledWith("guild-1");
    expect(result.current.notice).toBeNull();
  });

  it("reuses cached guild role options across mounts", async () => {
    const roles: GuildRoleOption[] = [
      {
        id: "role-a",
        name: "Alpha",
        position: 2,
        managed: false,
        is_default: false,
      },
    ];
    mockDashboardSession.client.listGuildRoleOptions.mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      roles,
    });

    const firstHook = renderHook(() => useGuildRoleOptions());
    await waitFor(() => {
      expect(firstHook.result.current.roles).toEqual(roles);
    });
    firstHook.unmount();

    const secondHook = renderHook(() => useGuildRoleOptions());

    expect(secondHook.result.current.roles).toEqual(roles);
    expect(
      mockDashboardSession.client.listGuildRoleOptions,
    ).toHaveBeenCalledTimes(1);
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

  it("reuses cached guild channel options across mounts", async () => {
    const channels: GuildChannelOption[] = [
      {
        id: "channel-a",
        name: "bot-commands",
        display_name: "#bot-commands",
        kind: "text",
        supports_message_route: true,
      },
    ];
    mockDashboardSession.client.listGuildChannelOptions.mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      channels,
    });

    const firstHook = renderHook(() => useGuildChannelOptions());
    await waitFor(() => {
      expect(firstHook.result.current.channels).toEqual(channels);
    });
    firstHook.unmount();

    const secondHook = renderHook(() => useGuildChannelOptions());

    expect(secondHook.result.current.channels).toEqual(channels);
    expect(
      mockDashboardSession.client.listGuildChannelOptions,
    ).toHaveBeenCalledTimes(1);
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

    expect(
      mockDashboardSession.client.listGuildMemberOptions,
    ).toHaveBeenCalledWith("guild-1", {
      query: "ali",
      selectedId: "user-2",
      limit: 25,
    });
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

  it("blocks guild feature patching when the selected server is read-only", async () => {
    mockDashboardSession.canEditSelectedGuild = false;

    const onError = vi.fn();
    const { result } = renderHook(() =>
      useFeatureMutation({
        scope: "guild",
        onError,
      }),
    );

    let response: FeatureRecord | null = null;
    await act(async () => {
      response = await result.current.patchFeature("logging.member_join", {
        enabled: true,
      });
    });

    expect(response).toBeNull();
    expect(mockDashboardSession.client.patchGuildFeature).not.toHaveBeenCalled();
    expect(result.current.notice).toEqual({
      tone: "info",
      message: "You have read-only access to this server.",
    });
    expect(onError).toHaveBeenCalledWith(
      "You have read-only access to this server.",
    );
  });

  it("patches a guild feature with role_id details", async () => {
    const updatedFeature = buildFeatureRecord({
      id: "moderation.mute_role",
      category: "moderation",
      label: "Mute role",
      details: {
        role_id: "mute-role",
      },
      editable_fields: ["enabled", "role_id"],
    });
    mockDashboardSession.client.patchGuildFeature.mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      feature: updatedFeature,
    });

    const { result } = renderHook(() =>
      useFeatureMutation({
        scope: "guild",
      }),
    );

    let response: FeatureRecord | null = null;
    await act(async () => {
      response = await result.current.patchFeature("moderation.mute_role", {
        role_id: "mute-role",
      });
    });

    expect(mockDashboardSession.client.patchGuildFeature).toHaveBeenCalledWith(
      "guild-1",
      "moderation.mute_role",
      {
        role_id: "mute-role",
      },
    );
    expect(response).toEqual(updatedFeature);
    expect(result.current.notice).toBeNull();
  });

  it("patches a guild stats feature with config_enabled and update_interval_mins", async () => {
    const updatedFeature = buildFeatureRecord({
      id: "stats_channels",
      category: "stats",
      label: "Stats channels",
      description: "Periodic member-count channel updates",
      details: {
        config_enabled: true,
        update_interval_mins: 45,
        configured_channel_count: 2,
        channels: [
          {
            channel_id: "stats-total",
            label: "Total members",
            member_type: "all",
          },
          {
            channel_id: "stats-bots",
            label: "Bot count",
            member_type: "bots",
          },
        ],
      },
      editable_fields: ["enabled", "config_enabled", "update_interval_mins"],
    });
    mockDashboardSession.client.patchGuildFeature.mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      feature: updatedFeature,
    });

    const { result } = renderHook(() =>
      useFeatureMutation({
        scope: "guild",
      }),
    );

    let response: FeatureRecord | null = null;
    await act(async () => {
      response = await result.current.patchFeature("stats_channels", {
        config_enabled: true,
        update_interval_mins: 45,
      });
    });

    expect(mockDashboardSession.client.patchGuildFeature).toHaveBeenCalledWith(
      "guild-1",
      "stats_channels",
      {
        config_enabled: true,
        update_interval_mins: 45,
      },
    );
    expect(response).toEqual(updatedFeature);
    expect(result.current.notice).toBeNull();
  });

  it("patches a guild backfill feature with channel_id and schedule seed", async () => {
    const updatedFeature = buildFeatureRecord({
      id: "backfill.enabled",
      category: "backfill",
      label: "Entry/exit backfill",
      description: "Backfill entry and exit metrics",
      details: {
        channel_id: "ops-commands",
        start_day: "2026-03-01",
        initial_date: "",
      },
      editable_fields: ["enabled", "channel_id", "start_day", "initial_date"],
    });
    mockDashboardSession.client.patchGuildFeature.mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      feature: updatedFeature,
    });

    const { result } = renderHook(() =>
      useFeatureMutation({
        scope: "guild",
      }),
    );

    let response: FeatureRecord | null = null;
    await act(async () => {
      response = await result.current.patchFeature("backfill.enabled", {
        channel_id: "ops-commands",
        start_day: "2026-03-01",
        initial_date: "",
      });
    });

    expect(mockDashboardSession.client.patchGuildFeature).toHaveBeenCalledWith(
      "guild-1",
      "backfill.enabled",
      {
        channel_id: "ops-commands",
        start_day: "2026-03-01",
        initial_date: "",
      },
    );
    expect(response).toEqual(updatedFeature);
    expect(result.current.notice).toBeNull();
  });

  it("patches a guild user prune feature with prune config details", async () => {
    const updatedFeature = buildFeatureRecord({
      id: "user_prune",
      category: "maintenance",
      label: "User prune",
      description: "Periodic user prune workflow",
      details: {
        config_enabled: false,
        grace_days: 45,
        scan_interval_mins: 90,
        initial_delay_secs: 20,
        kicks_per_second: 3,
        max_kicks_per_run: 40,
        exempt_role_ids: ["role-guard", "role-target"],
        exempt_role_count: 2,
        dry_run: false,
      },
      editable_fields: [
        "enabled",
        "config_enabled",
        "grace_days",
        "scan_interval_mins",
        "initial_delay_secs",
        "kicks_per_second",
        "max_kicks_per_run",
        "exempt_role_ids",
        "dry_run",
      ],
    });
    mockDashboardSession.client.patchGuildFeature.mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      feature: updatedFeature,
    });

    const { result } = renderHook(() =>
      useFeatureMutation({
        scope: "guild",
      }),
    );

    let response: FeatureRecord | null = null;
    await act(async () => {
      response = await result.current.patchFeature("user_prune", {
        config_enabled: false,
        grace_days: 45,
        scan_interval_mins: 90,
        initial_delay_secs: 20,
        kicks_per_second: 3,
        max_kicks_per_run: 40,
        exempt_role_ids: ["role-guard", "role-target"],
        dry_run: false,
      });
    });

    expect(mockDashboardSession.client.patchGuildFeature).toHaveBeenCalledWith(
      "guild-1",
      "user_prune",
      {
        config_enabled: false,
        grace_days: 45,
        scan_interval_mins: 90,
        initial_delay_secs: 20,
        kicks_per_second: 3,
        max_kicks_per_run: 40,
        exempt_role_ids: ["role-guard", "role-target"],
        dry_run: false,
      },
    );
    expect(response).toEqual(updatedFeature);
    expect(result.current.notice).toBeNull();
  });
});
