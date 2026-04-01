import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import App from "./App";
import { appRoutes } from "./app/routes";
import type {
  GuildChannelOption,
  FeatureRecord,
  GuildMemberOption,
  GuildRoleOption,
  PartnerBoardConfig,
} from "./api/control";

const testGuildID = "guild-1";
const testRoutes = {
  home: appRoutes.dashboardHome(testGuildID),
  coreControlPanel: appRoutes.dashboardCoreControlPanel(testGuildID),
  moderation: appRoutes.dashboardModerationModeration(testGuildID),
  rolesAutorole: appRoutes.dashboardRolesAutorole(testGuildID),
  coreCommands: appRoutes.dashboardCoreCommands(testGuildID),
  moderationLogging: appRoutes.dashboardModerationLogging(testGuildID),
  coreStats: appRoutes.dashboardCoreStats(testGuildID),
};

async function selectServerOption(name: string) {
  await userEvent.click(await screen.findByRole("button", { name: "Server" }));
  await userEvent.click(
    await screen.findByRole("menuitem", {
      name: new RegExp(name, "i"),
    }),
  );
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

function createFetchMock() {
  const boardCalls: string[] = [];
  const featureCalls: string[] = [];
  const featureUpdates: Array<{
    guildID: string;
    featureID: string;
    payload: Record<string, unknown>;
  }> = [];
  const targetUpdates: Array<{
    guildID: string;
    payload: Record<string, unknown>;
  }> = [];
  const settingsUpdates: Array<{
    guildID: string;
    payload: Record<string, unknown>;
  }> = [];
  const guildSettingsByGuild: Record<
    string,
    {
      roles: {
        allowed: string[];
        dashboard_read: string[];
        dashboard_write: string[];
      };
    }
  > = {
    "guild-1": {
      roles: {
        allowed: ["role-guard"],
        dashboard_read: ["role-target"],
        dashboard_write: ["role-guard"],
      },
    },
    "guild-2": {
      roles: {
        allowed: [],
        dashboard_read: [],
        dashboard_write: [],
      },
    },
  };
  const roleOptionsByGuild: Record<string, GuildRoleOption[]> = {
    "guild-1": [
      {
        id: "guild-1",
        name: "@everyone",
        position: 0,
        managed: false,
        is_default: true,
      },
      {
        id: "role-target",
        name: "Members",
        position: 4,
        managed: false,
        is_default: false,
      },
      {
        id: "role-level",
        name: "Level Five",
        position: 3,
        managed: false,
        is_default: false,
      },
      {
        id: "role-booster",
        name: "Boosters",
        position: 2,
        managed: false,
        is_default: false,
      },
      {
        id: "role-guard",
        name: "Moderators",
        position: 1,
        managed: false,
        is_default: false,
      },
      {
        id: "mute-role",
        name: "Muted",
        position: 1,
        managed: false,
        is_default: false,
      },
    ],
    "guild-2": [
      {
        id: "guild-2",
        name: "@everyone",
        position: 0,
        managed: false,
        is_default: true,
      },
    ],
  };
  const channelOptionsByGuild: Record<string, GuildChannelOption[]> = {
    "guild-1": [
      {
        id: "bot-commands",
        name: "bot-commands",
        display_name: "#bot-commands",
        kind: "text",
        supports_message_route: true,
      },
      {
        id: "ops-commands",
        name: "ops-commands",
        display_name: "#ops-commands",
        kind: "text",
        supports_message_route: true,
      },
      {
        id: "user-log-channel",
        name: "user-log-channel",
        display_name: "#user-log-channel",
        kind: "text",
        supports_message_route: true,
      },
      {
        id: "join-channel",
        name: "join-channel",
        display_name: "#join-channel",
        kind: "text",
        supports_message_route: true,
      },
      {
        id: "mod-actions",
        name: "mod-actions",
        display_name: "#mod-actions",
        kind: "text",
        supports_message_route: true,
      },
      {
        id: "mod-cases",
        name: "mod-cases",
        display_name: "#mod-cases",
        kind: "text",
        supports_message_route: true,
      },
      {
        id: "voice-hub",
        name: "Voice Hub",
        display_name: "Voice Hub",
        kind: "voice",
        supports_message_route: false,
      },
      {
        id: "stats-total",
        name: "Total Members",
        display_name: "Total Members",
        kind: "voice",
        supports_message_route: false,
      },
      {
        id: "stats-bots",
        name: "Bot Count",
        display_name: "Bot Count",
        kind: "voice",
        supports_message_route: false,
      },
    ],
    "guild-2": [
      {
        id: "guild-2-general",
        name: "general",
        display_name: "#general",
        kind: "text",
        supports_message_route: true,
      },
    ],
  };
  const memberOptionsByGuild: Record<string, GuildMemberOption[]> = {
    "guild-1": [
      {
        id: "user-alice",
        display_name: "Alice Alpha",
        username: "alice",
        bot: false,
      },
      {
        id: "user-bob",
        display_name: "Bob Beta",
        username: "bob",
        bot: false,
      },
      {
        id: "user-carol",
        display_name: "Carol Gamma",
        username: "carol",
        bot: false,
      },
      {
        id: "bot-helper",
        display_name: "Helper Bot",
        username: "helperbot",
        bot: true,
      },
    ],
    "guild-2": [
      {
        id: "user-delta",
        display_name: "Delta",
        username: "delta",
        bot: false,
      },
    ],
  };
  const boardByGuild: Record<string, PartnerBoardConfig> = {
    "guild-1": {
      target: {
        type: "channel_message",
        message_id: "111111111111111111",
        channel_id: "222222222222222222",
      },
      template: {
        title: "Partner Board",
        intro: "Server one intro",
        section_header_template: "Section header",
        line_template: "Partner row",
        empty_state_text: "No partners yet",
      },
      partners: [
        {
          fandom: "Action",
          name: "Server One",
          link: "https://discord.gg/server-one",
        },
      ],
    },
    "guild-2": {
      target: {
        type: "webhook_message",
        message_id: "333333333333333333",
        webhook_url: "https://discord.com/api/webhooks/example",
      },
      template: {
        title: "Partner Board",
        intro: "Server two intro",
        section_header_template: "Section header",
        line_template: "Partner row",
        empty_state_text: "No partners yet",
      },
      partners: [
        {
          fandom: "Puzzle",
          name: "Server Two",
          link: "https://discord.gg/server-two",
        },
      ],
    },
    "guild-3": {
      target: {
        type: "channel_message",
        message_id: "444444444444444444",
        channel_id: "555555555555555555",
      },
      template: {
        title: "Partner Board",
        intro: "Server three intro",
        section_header_template: "Section header",
        line_template: "Partner row",
        empty_state_text: "No partners yet",
      },
      partners: [
        {
          fandom: "Rhythm",
          name: "Server Three",
          link: "https://discord.gg/server-three",
        },
      ],
    },
  };
  const featuresByGuild: Record<string, FeatureRecord[]> = {
    "guild-1": [
      {
        id: "services.monitoring",
        category: "services",
        label: "Monitoring",
        description: "Core monitoring",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
      },
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
      {
        id: "services.automod",
        category: "services",
        label: "Automod service",
        description: "Discord native AutoMod listener used for logging.",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
        details: {
          mode: "logging_only",
        },
        editable_fields: ["enabled"],
      },
      {
        id: "moderation.mute_role",
        category: "moderation",
        label: "Mute role",
        description: "Role applied by the mute command.",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "blocked",
        blockers: [
          {
            code: "missing_role",
            message:
              "Choose the role that should be applied by the mute command.",
            field: "role_id",
          },
        ],
        details: {
          role_id: "",
        },
        editable_fields: ["enabled", "role_id"],
      },
      {
        id: "logging.automod_action",
        category: "logging",
        label: "Automod action logging",
        description: "AutoMod executions",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
        details: {
          requires_channel: true,
          channel_id: "mod-actions",
          validate_channel_permissions: true,
          exclusive_moderation_channel: true,
          runtime_toggle_path: "disable_automod_logs",
        },
        editable_fields: ["enabled", "channel_id"],
      },
      {
        id: "logging.moderation_case",
        category: "logging",
        label: "Moderation case logging",
        description: "Moderation cases",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "blocked",
        blockers: [
          {
            code: "missing_channel",
            message: "Choose a channel for this log route.",
            field: "channel_id",
          },
        ],
        details: {
          requires_channel: true,
          channel_id: "",
          validate_channel_permissions: true,
          exclusive_moderation_channel: true,
          runtime_toggle_path: "moderation_logging",
        },
        editable_fields: ["enabled", "channel_id"],
      },
      {
        id: "logging.avatar_logging",
        category: "logging",
        label: "Avatar logging",
        description: "Avatar changes",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
        details: {
          requires_channel: true,
          channel_id: "user-log-channel",
          validate_channel_permissions: true,
          runtime_toggle_path: "disable_user_logs",
        },
        editable_fields: ["enabled", "channel_id"],
      },
      {
        id: "logging.member_join",
        category: "logging",
        label: "Member join logging",
        description: "Member joins",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "blocked",
        blockers: [
          {
            code: "missing_channel",
            message: "Choose a channel for member join logs.",
          },
        ],
        details: {
          requires_channel: true,
          validate_channel_permissions: true,
          runtime_toggle_path: "disable_entry_exit_logs",
        },
        editable_fields: ["enabled", "channel_id"],
      },
      {
        id: "presence_watch.bot",
        category: "presence_watch",
        label: "Presence watch (bot)",
        description: "Presence watch for the bot identity",
        scope: "guild",
        supports_guild_override: true,
        override_state: "disabled",
        effective_enabled: false,
        effective_source: "guild",
        readiness: "disabled",
        details: {
          watch_bot: false,
        },
        editable_fields: ["enabled", "watch_bot"],
      },
      {
        id: "presence_watch.user",
        category: "presence_watch",
        label: "Presence watch (user)",
        description: "Presence watch",
        scope: "guild",
        supports_guild_override: true,
        override_state: "disabled",
        effective_enabled: false,
        effective_source: "guild",
        readiness: "disabled",
        details: {
          user_id: "",
        },
        editable_fields: ["enabled", "user_id"],
      },
      {
        id: "safety.bot_role_perm_mirror",
        category: "safety",
        label: "Bot role permission mirror",
        description: "Permission mirror guard",
        scope: "guild",
        supports_guild_override: true,
        override_state: "disabled",
        effective_enabled: false,
        effective_source: "guild",
        readiness: "disabled",
        details: {
          actor_role_id: "",
          runtime_disabled: false,
        },
        editable_fields: ["enabled", "actor_role_id"],
      },
      {
        id: "auto_role_assignment",
        category: "roles",
        label: "Auto role assignment",
        description: "Auto role",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "blocked",
        blockers: [
          {
            code: "config_disabled",
            message: "Auto assignment config is disabled.",
            field: "config_enabled",
          },
        ],
        details: {
          config_enabled: false,
          target_role_id: "",
          required_role_ids: [],
          required_role_count: 0,
          level_role_id: "",
          booster_role_id: "",
        },
        editable_fields: [
          "enabled",
          "config_enabled",
          "target_role_id",
          "required_role_ids",
        ],
      },
      {
        id: "message_cache.cleanup_on_startup",
        category: "message_cache",
        label: "Message cache cleanup",
        description:
          "Allow startup cleanup when the runtime switch is enabled.",
        scope: "guild",
        supports_guild_override: true,
        override_state: "inherit",
        effective_enabled: false,
        effective_source: "global",
        readiness: "disabled",
        editable_fields: ["enabled"],
      },
      {
        id: "message_cache.delete_on_log",
        category: "message_cache",
        label: "Delete on log",
        description:
          "Delete cached messages after logging when the runtime switch is enabled.",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
        editable_fields: ["enabled"],
      },
      {
        id: "maintenance.db_cleanup",
        category: "maintenance",
        label: "Database cleanup",
        description: "Cleanup",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
        editable_fields: ["enabled"],
      },
      {
        id: "backfill.enabled",
        category: "backfill",
        label: "Entry/exit backfill",
        description:
          "Backfill entry and exit metrics when routing and runtime dates are configured.",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "blocked",
        blockers: [
          {
            code: "missing_channel",
            message: "Backfill needs a configured source channel.",
            field: "channel_id",
          },
        ],
        details: {
          channel_id: "",
          start_day: "",
          initial_date: "",
        },
        editable_fields: ["enabled", "channel_id", "start_day", "initial_date"],
      },
      {
        id: "user_prune",
        category: "maintenance",
        label: "User prune",
        description:
          "Periodic user prune workflow plus its guild-level pruning configuration.",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
        details: {
          config_enabled: true,
          grace_days: 30,
          scan_interval_mins: 60,
          initial_delay_secs: 15,
          kicks_per_second: 2,
          max_kicks_per_run: 25,
          exempt_role_ids: ["role-guard"],
          exempt_role_count: 1,
          dry_run: true,
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
      },
      {
        id: "stats_channels",
        category: "stats",
        label: "Stats channels",
        description:
          "Periodic member-count channel updates driven by configured stats channels.",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "blocked",
        blockers: [
          {
            code: "config_disabled",
            message: "Stats channel config is disabled.",
            field: "config_enabled",
          },
        ],
        details: {
          config_enabled: false,
          update_interval_mins: 30,
          configured_channel_count: 2,
          channels: [
            {
              channel_id: "stats-total",
              label: "Total members",
              name_template: "",
              member_type: "all",
            },
            {
              channel_id: "stats-bots",
              label: "Bot count",
              name_template: "{label} | {count}",
              member_type: "bots",
            },
          ],
        },
        editable_fields: ["enabled", "config_enabled", "update_interval_mins"],
      },
    ],
    "guild-2": [
      {
        id: "services.monitoring",
        category: "services",
        label: "Monitoring",
        description: "Core monitoring",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
      },
    ],
  };

  function evaluateFeatureState(feature: FeatureRecord) {
    const requiresChannel =
      feature.details !== undefined &&
      feature.details !== null &&
      feature.details.requires_channel === true;
    const channelId =
      feature.details !== undefined &&
      feature.details !== null &&
      typeof feature.details.channel_id === "string"
        ? feature.details.channel_id.trim()
        : "";

    if (!feature.effective_enabled) {
      feature.readiness = "disabled";
      feature.blockers = [];
      return;
    }

    if (requiresChannel && channelId === "") {
      feature.readiness = "blocked";
      feature.blockers = [
        {
          code: "missing_channel",
          message: "Choose a channel for this log route.",
          field: "channel_id",
        },
      ];
      return;
    }

    if (feature.id === "moderation.mute_role") {
      const roleId =
        typeof feature.details?.role_id === "string"
          ? feature.details.role_id.trim()
          : "";

      if (roleId === "") {
        feature.readiness = "blocked";
        feature.blockers = [
          {
            code: "missing_role",
            message:
              "Choose the role that should be applied by the mute command.",
            field: "role_id",
          },
        ];
        return;
      }

      const guildRoleOptions = roleOptionsByGuild["guild-1"] ?? [];
      if (!guildRoleOptions.some((role) => role.id === roleId)) {
        feature.readiness = "blocked";
        feature.blockers = [
          {
            code: "invalid_role",
            message:
              "The configured mute role is no longer available in this server.",
            field: "role_id",
          },
        ];
        return;
      }
    }

    if (feature.id === "services.automod") {
      feature.readiness = "ready";
      feature.blockers = [];
      return;
    }

    if (feature.id === "auto_role_assignment") {
      const configEnabled = feature.details?.config_enabled === true;
      const targetRoleId =
        typeof feature.details?.target_role_id === "string"
          ? feature.details.target_role_id.trim()
          : "";
      const requiredRoleIds = Array.isArray(feature.details?.required_role_ids)
        ? feature.details.required_role_ids.filter(
            (value): value is string =>
              typeof value === "string" && value.trim() !== "",
          )
        : [];

      if (!configEnabled) {
        feature.readiness = "blocked";
        feature.blockers = [
          {
            code: "config_disabled",
            message: "Auto assignment config is disabled.",
            field: "config_enabled",
          },
        ];
        return;
      }

      if (targetRoleId === "") {
        feature.readiness = "blocked";
        feature.blockers = [
          {
            code: "missing_target_role",
            message: "Auto assignment needs a target role.",
            field: "target_role_id",
          },
        ];
        return;
      }

      if (requiredRoleIds.length !== 2) {
        feature.readiness = "blocked";
        feature.blockers = [
          {
            code: "invalid_required_roles",
            message:
              "Auto assignment needs exactly two required roles in order.",
            field: "required_role_ids",
          },
        ];
        return;
      }
    }

    if (feature.id === "stats_channels") {
      const configEnabled = feature.details?.config_enabled === true;
      const configuredChannelCount =
        typeof feature.details?.configured_channel_count === "number" &&
        Number.isFinite(feature.details.configured_channel_count)
          ? feature.details.configured_channel_count
          : Array.isArray(feature.details?.channels)
            ? feature.details.channels.filter(
                (value): value is Record<string, unknown> =>
                  typeof value === "object" &&
                  value !== null &&
                  typeof value.channel_id === "string" &&
                  value.channel_id.trim() !== "",
              ).length
            : 0;

      if (!configEnabled) {
        feature.readiness = "blocked";
        feature.blockers = [
          {
            code: "config_disabled",
            message: "Stats channel config is disabled.",
            field: "config_enabled",
          },
        ];
        return;
      }

      if (configuredChannelCount === 0) {
        feature.readiness = "blocked";
        feature.blockers = [
          {
            code: "missing_channels",
            message: "Stats channels need at least one configured target.",
          },
        ];
        return;
      }
    }

    if (feature.id === "backfill.enabled") {
      const startDay =
        typeof feature.details?.start_day === "string"
          ? feature.details.start_day.trim()
          : "";
      const initialDate =
        typeof feature.details?.initial_date === "string"
          ? feature.details.initial_date.trim()
          : "";

      if (channelId === "") {
        feature.readiness = "blocked";
        feature.blockers = [
          {
            code: "missing_channel",
            message: "Backfill needs a configured source channel.",
            field: "channel_id",
          },
        ];
        return;
      }

      if (startDay === "" && initialDate === "") {
        feature.readiness = "blocked";
        feature.blockers = [
          {
            code: "missing_schedule_seed",
            message: "Backfill needs start_day or initial_date configured.",
            field: "start_day",
          },
        ];
        return;
      }
    }

    if (feature.id === "user_prune") {
      const configEnabled = feature.details?.config_enabled === true;

      if (!configEnabled) {
        feature.readiness = "blocked";
        feature.blockers = [
          {
            code: "config_disabled",
            message: "User prune config is disabled.",
            field: "config_enabled",
          },
        ];
        return;
      }
    }

    if (
      feature.id === "presence_watch.bot" &&
      feature.details?.watch_bot !== true
    ) {
      feature.readiness = "blocked";
      feature.blockers = [
        {
          code: "runtime_disabled",
          message: "Runtime bot presence watching is disabled.",
          field: "watch_bot",
        },
      ];
      return;
    }

    if (
      feature.id === "presence_watch.user" &&
      typeof feature.details?.user_id === "string" &&
      feature.details.user_id.trim() === ""
    ) {
      feature.readiness = "blocked";
      feature.blockers = [
        {
          code: "missing_user_id",
          message: "Presence watch needs a user ID.",
          field: "user_id",
        },
      ];
      return;
    }

    if (
      feature.id === "safety.bot_role_perm_mirror" &&
      typeof feature.details?.actor_role_id === "string" &&
      feature.details.actor_role_id.trim() === "missing-role"
    ) {
      feature.readiness = "blocked";
      feature.blockers = [
        {
          code: "invalid_actor_role",
          message:
            "Permission mirror actor role is no longer available in this server.",
          field: "actor_role_id",
        },
      ];
      return;
    }

    feature.readiness = "ready";
    feature.blockers = [];
  }

  function buildGuildSettingsWorkspace(guildID: string) {
    const settings = guildSettingsByGuild[guildID] ?? {
      roles: {
        allowed: [],
        dashboard_read: [],
        dashboard_write: [],
      },
    };

    return {
      status: "ok",
      workspace: {
        scope: "guild",
        guild_id: guildID,
        sections: {
          roles: {
            allowed: [...settings.roles.allowed],
            dashboard_read: [...settings.roles.dashboard_read],
            dashboard_write: [...settings.roles.dashboard_write],
          },
        },
      },
    };
  }

  const fetchMock = vi.fn(
    async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();

      if (url.endsWith("/auth/me")) {
        return jsonResponse({
          status: "ok",
          user: {
            id: "user-1",
            username: "alice",
            global_name: "alice",
          },
          scopes: ["identify", "guilds"],
          csrf_token: "csrf-token",
          expires_at: "2099-01-01T00:00:00Z",
        });
      }

      if (url.endsWith("/auth/guilds/access")) {
        return jsonResponse({
          status: "ok",
          count: 3,
          guilds: [
            {
              id: "guild-1",
              name: "Server One",
              owner: true,
              permissions: 8,
              access_level: "write",
            },
            {
              id: "guild-2",
              name: "Server Two",
              owner: false,
              permissions: 32,
              access_level: "read",
            },
            {
              id: "guild-3",
              name: "Server Three",
              owner: false,
              permissions: 32,
              access_level: "write",
            },
          ],
        });
      }

      if (url.endsWith("/auth/guilds/manageable")) {
        return jsonResponse({
          status: "ok",
          count: 2,
          guilds: [
            {
              id: "guild-1",
              name: "Server One",
              owner: true,
              permissions: 8,
              access_level: "write",
            },
            {
              id: "guild-3",
              name: "Server Three",
              owner: false,
              permissions: 32,
              access_level: "write",
            },
          ],
        });
      }

      if (url.includes("/settings")) {
        const match = url.match(/\/v1\/guilds\/([^/]+)\/settings$/);
        if (match && (!init?.method || init.method === "GET")) {
          const guildID = decodeURIComponent(match[1] ?? "");
          return jsonResponse(buildGuildSettingsWorkspace(guildID));
        }

        if (match && init?.method === "PUT") {
          const guildID = decodeURIComponent(match[1] ?? "");
          const payload = JSON.parse(String(init.body)) as Record<string, unknown>;
          settingsUpdates.push({ guildID, payload });
          const rolesPayload =
            typeof payload.roles === "object" && payload.roles !== null
              ? (payload.roles as Record<string, unknown>)
              : {};
          guildSettingsByGuild[guildID] = {
            roles: {
              allowed: guildSettingsByGuild[guildID]?.roles.allowed ?? [],
              dashboard_read: Array.isArray(rolesPayload.dashboard_read)
                ? rolesPayload.dashboard_read.filter(
                    (value): value is string => typeof value === "string",
                  )
                : [],
              dashboard_write: Array.isArray(rolesPayload.dashboard_write)
                ? rolesPayload.dashboard_write.filter(
                    (value): value is string => typeof value === "string",
                  )
                : [],
            },
          };
          return jsonResponse(buildGuildSettingsWorkspace(guildID));
        }
      }

      if (url.includes("/role-options")) {
        const match = url.match(/\/v1\/guilds\/([^/]+)\/role-options$/);
        if (match) {
          const guildID = decodeURIComponent(match[1] ?? "");
          return jsonResponse({
            status: "ok",
            guild_id: guildID,
            roles: roleOptionsByGuild[guildID] ?? [],
          });
        }
      }

      if (url.includes("/channel-options")) {
        const match = url.match(/\/v1\/guilds\/([^/]+)\/channel-options$/);
        if (match) {
          const guildID = decodeURIComponent(match[1] ?? "");
          return jsonResponse({
            status: "ok",
            guild_id: guildID,
            channels: channelOptionsByGuild[guildID] ?? [],
          });
        }
      }

      if (url.includes("/member-options")) {
        const parsed = new URL(url, "http://localhost");
        const match = parsed.pathname.match(
          /\/v1\/guilds\/([^/]+)\/member-options$/,
        );
        if (match) {
          const guildID = decodeURIComponent(match[1] ?? "");
          const query = (parsed.searchParams.get("query") ?? "")
            .trim()
            .toLowerCase();
          const selectedID = (
            parsed.searchParams.get("selected_id") ?? ""
          ).trim();
          const limit = Number(parsed.searchParams.get("limit") ?? "25");
          const allMembers = memberOptionsByGuild[guildID] ?? [];

          const selectedMember =
            selectedID === ""
              ? null
              : (allMembers.find((member) => member.id === selectedID) ?? null);
          const matches = allMembers.filter((member) => {
            if (query === "") {
              return true;
            }
            return (
              member.display_name.toLowerCase().startsWith(query) ||
              member.username.toLowerCase().startsWith(query) ||
              member.id.toLowerCase().startsWith(query)
            );
          });
          const members = [
            ...(selectedMember === null ? [] : [selectedMember]),
            ...matches.filter((member) => member.id !== selectedID),
          ].slice(0, Number.isFinite(limit) && limit > 0 ? limit : 25);

          return jsonResponse({
            status: "ok",
            guild_id: guildID,
            members,
          });
        }
      }

      if (url.includes("/features/") && init?.method === "PATCH") {
        const match = url.match(/\/v1\/guilds\/([^/]+)\/features\/([^/?]+)/);
        if (match) {
          const guildID = decodeURIComponent(match[1] ?? "");
          const featureID = decodeURIComponent(match[2] ?? "");
          const payload = JSON.parse(String(init.body)) as Record<
            string,
            unknown
          >;
          featureUpdates.push({
            guildID,
            featureID,
            payload,
          });

          const feature = featuresByGuild[guildID]?.find(
            (item) => item.id === featureID,
          );
          if (feature === undefined) {
            return new Response("not found", { status: 404 });
          }

          if (Object.prototype.hasOwnProperty.call(payload, "enabled")) {
            const enabled = payload.enabled;
            if (enabled === null) {
              feature.override_state = "inherit";
              feature.effective_enabled = false;
              feature.effective_source = "global";
            } else {
              const nextEnabled = Boolean(enabled);
              feature.override_state = nextEnabled ? "enabled" : "disabled";
              feature.effective_enabled = nextEnabled;
              feature.effective_source = "guild";
            }
          }

          if (Object.prototype.hasOwnProperty.call(payload, "channel_id")) {
            feature.details = {
              ...(feature.details ?? {}),
              channel_id: String(payload.channel_id ?? ""),
            };
          }

          if (Object.prototype.hasOwnProperty.call(payload, "role_id")) {
            feature.details = {
              ...(feature.details ?? {}),
              role_id: String(payload.role_id ?? ""),
            };
          }

          if (
            Object.prototype.hasOwnProperty.call(payload, "allowed_role_ids")
          ) {
            const allowedRoleIDs = Array.isArray(payload.allowed_role_ids)
              ? payload.allowed_role_ids
                  .filter((value): value is string => typeof value === "string")
                  .map((value) => value.trim())
                  .filter((value) => value !== "")
              : [];
            feature.details = {
              ...(feature.details ?? {}),
              allowed_role_ids: allowedRoleIDs,
              allowed_role_count: allowedRoleIDs.length,
            };
          }

          if (Object.prototype.hasOwnProperty.call(payload, "config_enabled")) {
            feature.details = {
              ...(feature.details ?? {}),
              config_enabled: Boolean(payload.config_enabled),
            };
          }

          if (
            Object.prototype.hasOwnProperty.call(
              payload,
              "update_interval_mins",
            )
          ) {
            feature.details = {
              ...(feature.details ?? {}),
              update_interval_mins: Number(payload.update_interval_mins ?? 0),
            };
          }

          if (Object.prototype.hasOwnProperty.call(payload, "start_day")) {
            feature.details = {
              ...(feature.details ?? {}),
              start_day: String(payload.start_day ?? ""),
            };
          }

          if (Object.prototype.hasOwnProperty.call(payload, "initial_date")) {
            feature.details = {
              ...(feature.details ?? {}),
              initial_date: String(payload.initial_date ?? ""),
            };
          }

          if (Object.prototype.hasOwnProperty.call(payload, "grace_days")) {
            feature.details = {
              ...(feature.details ?? {}),
              grace_days: Number(payload.grace_days ?? 0),
            };
          }

          if (
            Object.prototype.hasOwnProperty.call(payload, "scan_interval_mins")
          ) {
            feature.details = {
              ...(feature.details ?? {}),
              scan_interval_mins: Number(payload.scan_interval_mins ?? 0),
            };
          }

          if (
            Object.prototype.hasOwnProperty.call(payload, "initial_delay_secs")
          ) {
            feature.details = {
              ...(feature.details ?? {}),
              initial_delay_secs: Number(payload.initial_delay_secs ?? 0),
            };
          }

          if (
            Object.prototype.hasOwnProperty.call(payload, "kicks_per_second")
          ) {
            feature.details = {
              ...(feature.details ?? {}),
              kicks_per_second: Number(payload.kicks_per_second ?? 0),
            };
          }

          if (
            Object.prototype.hasOwnProperty.call(payload, "max_kicks_per_run")
          ) {
            feature.details = {
              ...(feature.details ?? {}),
              max_kicks_per_run: Number(payload.max_kicks_per_run ?? 0),
            };
          }

          if (
            Object.prototype.hasOwnProperty.call(payload, "exempt_role_ids")
          ) {
            const exemptRoleIDs = Array.isArray(payload.exempt_role_ids)
              ? payload.exempt_role_ids
                  .filter((value): value is string => typeof value === "string")
                  .map((value) => value.trim())
                  .filter((value) => value !== "")
              : [];
            feature.details = {
              ...(feature.details ?? {}),
              exempt_role_ids: exemptRoleIDs,
              exempt_role_count: exemptRoleIDs.length,
            };
          }

          if (Object.prototype.hasOwnProperty.call(payload, "dry_run")) {
            feature.details = {
              ...(feature.details ?? {}),
              dry_run: Boolean(payload.dry_run),
            };
          }

          if (Object.prototype.hasOwnProperty.call(payload, "target_role_id")) {
            feature.details = {
              ...(feature.details ?? {}),
              target_role_id: String(payload.target_role_id ?? ""),
            };
          }

          if (
            Object.prototype.hasOwnProperty.call(payload, "required_role_ids")
          ) {
            const requiredRoleIDs = Array.isArray(payload.required_role_ids)
              ? payload.required_role_ids
                  .filter((value): value is string => typeof value === "string")
                  .map((value) => value.trim())
                  .filter((value) => value !== "")
              : [];
            feature.details = {
              ...(feature.details ?? {}),
              required_role_ids: requiredRoleIDs,
              required_role_count: requiredRoleIDs.length,
              level_role_id: requiredRoleIDs[0] ?? "",
              booster_role_id: requiredRoleIDs[1] ?? "",
            };
          }

          if (Object.prototype.hasOwnProperty.call(payload, "watch_bot")) {
            feature.details = {
              ...(feature.details ?? {}),
              watch_bot: Boolean(payload.watch_bot),
            };
          }

          if (Object.prototype.hasOwnProperty.call(payload, "user_id")) {
            feature.details = {
              ...(feature.details ?? {}),
              user_id: String(payload.user_id ?? ""),
            };
          }

          if (Object.prototype.hasOwnProperty.call(payload, "actor_role_id")) {
            feature.details = {
              ...(feature.details ?? {}),
              actor_role_id: String(payload.actor_role_id ?? ""),
            };
          }

          evaluateFeatureState(feature);

          return jsonResponse({
            status: "ok",
            guild_id: guildID,
            feature: {
              ...feature,
            },
          });
        }
      }

      if (url.includes("/features") && !url.includes("/catalog")) {
        const match = url.match(/\/v1\/guilds\/([^/]+)\/features$/);
        if (match) {
          const guildID = decodeURIComponent(match[1] ?? "");
          featureCalls.push(guildID);
          return jsonResponse({
            status: "ok",
            workspace: {
              scope: "guild",
              guild_id: guildID,
              features: featuresByGuild[guildID] ?? [],
            },
          });
        }
      }

      if (url.includes("/partner-board/target") && init?.method === "PUT") {
        const match = url.match(
          /\/v1\/guilds\/([^/]+)\/partner-board\/target$/,
        );
        if (match) {
          const guildID = decodeURIComponent(match[1] ?? "");
          const payload = JSON.parse(String(init.body)) as Record<
            string,
            unknown
          >;
          targetUpdates.push({ guildID, payload });
          const nextBoard = boardByGuild[guildID];
          nextBoard.target = {
            ...nextBoard.target,
            ...(payload as PartnerBoardConfig["target"]),
          };

          if (payload.type === "channel_message") {
            delete nextBoard.target?.webhook_url;
          }
          if (payload.type === "webhook_message") {
            delete nextBoard.target?.channel_id;
          }

          return jsonResponse({
            status: "ok",
            guild_id: guildID,
            target: nextBoard.target,
          });
        }
      }

      if (
        url.includes("/partner-board") &&
        !url.endsWith("/partner-board/sync")
      ) {
        const match = url.match(/\/v1\/guilds\/([^/]+)\/partner-board$/);
        if (match) {
          const guildID = decodeURIComponent(match[1] ?? "");
          boardCalls.push(guildID);
          return jsonResponse({
            status: "ok",
            guild_id: guildID,
            partner_board: boardByGuild[guildID],
          });
        }
      }

      if (url.endsWith("/partner-board/sync")) {
        return jsonResponse({
          status: "ok",
          guild_id: "guild-1",
          synced: true,
        });
      }

      return new Response("not found", { status: 404 });
    },
  );

  return {
    boardCalls,
    featureCalls,
    fetchMock,
    featureUpdates,
    settingsUpdates,
    targetUpdates,
  };
}

describe("dashboard routing and workspace", () => {
  beforeEach(() => {
    window.history.replaceState({}, "", appRoutes.controlPanel);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    window.history.replaceState({}, "", "/");
  });

  it("renders the lean shell, preserves the legacy control-panel redirect, and keeps global context in the top bar", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);

    await screen.findByRole("heading", { name: "Control Panel", level: 1 });
    expect(window.location.pathname).toBe(testRoutes.coreControlPanel);
    expect(document.querySelector("main.dashboard-layout-shell")).not.toBeNull();
    expect(document.querySelector("aside.dashboard-layout-sidebar")).not.toBeNull();
    expect(document.querySelector("[data-shell-topbar]")).not.toBeNull();
    expect(screen.getByLabelText("Server")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /alice/i })).toBeInTheDocument();
    const sidebarNav = screen.getByRole("navigation", {
      name: "Dashboard navigation",
    });
    expect(within(sidebarNav).getByRole("link", { name: "Home" })).toBeInTheDocument();
    const coreButton = within(sidebarNav).getByRole("button", { name: "Core" });
    const moderationButton = within(sidebarNav).getByRole("button", {
      name: "Moderation",
    });
    const partnersButton = within(sidebarNav).getByRole("button", {
      name: "Partners",
    });
    const rolesButton = within(sidebarNav).getByRole("button", { name: "Roles" });
    expect(coreButton).toHaveAttribute("aria-expanded", "true");
    expect(moderationButton).toHaveAttribute("aria-expanded", "false");
    expect(partnersButton).toHaveAttribute("aria-expanded", "false");
    expect(rolesButton).toHaveAttribute("aria-expanded", "false");
    expect(
      within(sidebarNav).getByRole("link", { name: "Control Panel" }),
    ).toBeInTheDocument();
    expect(within(sidebarNav).getByRole("link", { name: "Commands" })).toBeInTheDocument();
    expect(within(sidebarNav).getByRole("link", { name: "Stats" })).toBeInTheDocument();
    expect(
      within(sidebarNav).queryByRole("link", { name: "Logging" }),
    ).not.toBeInTheDocument();
    expect(
      within(sidebarNav).queryByRole("link", { name: "Partner Board" }),
    ).not.toBeInTheDocument();
    expect(
      within(sidebarNav).queryByRole("link", { name: "Autorole" }),
    ).not.toBeInTheDocument();

    await userEvent.click(moderationButton);

    expect(coreButton).toHaveAttribute("aria-expanded", "false");
    expect(moderationButton).toHaveAttribute("aria-expanded", "true");
    expect(
      within(sidebarNav).queryByRole("link", { name: "Control Panel" }),
    ).not.toBeInTheDocument();
    expect(within(sidebarNav).getByRole("link", { name: "Moderation" })).toBeInTheDocument();
    expect(within(sidebarNav).getByRole("link", { name: "Logging" })).toBeInTheDocument();
    expect(
      within(sidebarNav).queryByRole("link", { name: "Settings" }),
    ).not.toBeInTheDocument();
    await userEvent.click(partnersButton);

    expect(moderationButton).toHaveAttribute("aria-expanded", "false");
    expect(partnersButton).toHaveAttribute("aria-expanded", "true");
    expect(
      within(sidebarNav).getByRole("link", { name: "Partner Board" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Reconnect" }),
    ).not.toBeInTheDocument();
  });

  it("renders Home as a navigation index with category headings and route cards", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.home);

    render(<App />);

    await screen.findByRole("heading", { name: "Home", level: 1 });
    const sidebarNav = screen.getByRole("navigation", {
      name: "Dashboard navigation",
    });
    expect(screen.getByRole("link", { name: "Home" })).toHaveClass("is-active");
    expect(
      within(sidebarNav).getByRole("button", { name: "Core" }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(
      within(sidebarNav).getByRole("button", { name: "Moderation" }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(
      within(sidebarNav).getByRole("button", { name: "Partners" }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(
      within(sidebarNav).getByRole("button", { name: "Roles" }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(
      within(sidebarNav).queryByRole("link", { name: "Control Panel" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Core", level: 2 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Moderation", level: 2 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Partners", level: 2 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Roles", level: 2 })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Control Panel" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Stats" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Commands" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Logging" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Partner Board" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Autorole" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Level Roles" })).toBeInTheDocument();
    expect(await screen.findByText("Write roles: 1")).toBeInTheDocument();
    expect(await screen.findByText("Read roles: 1")).toBeInTheDocument();
    expect(await screen.findByText("Command channel: Configured")).toBeInTheDocument();
    expect(await screen.findByText("Automod service: Enabled")).toBeInTheDocument();
    expect(await screen.findByText("Status: In development")).toBeInTheDocument();
    expect(screen.queryByText("Product areas")).not.toBeInTheDocument();
    expect(screen.queryByText("Product area")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Browse the dashboard by area."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("Dashboard access roles and core panel permissions."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Current blockers", level: 2 }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Quick shortcuts", level: 2 }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Settings and diagnostics", level: 2 }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Refresh home" }),
    ).not.toBeInTheDocument();
  });

  it("collapses and expands the sidebar from the navigation chrome", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.home);

    render(<App />);

    await screen.findByRole("heading", { name: "Home", level: 1 });
    expect(screen.getByRole("link", { name: "Home" })).toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: "Collapse navigation" }),
    );
    expect(screen.queryByRole("link", { name: "Home" })).not.toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: "Expand navigation" }),
    );
    expect(await screen.findByRole("link", { name: "Home" })).toBeInTheDocument();
  });

  it("saves dashboard read/write access roles from the dedicated Control Panel page", async () => {
    const { fetchMock, settingsUpdates } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", appRoutes.controlPanel);

    render(<App />);

    await screen.findByRole("heading", { name: "Control Panel", level: 1 });
    const writeAccessGroup = await screen.findByRole("group", {
      name: "Write access roles",
    });
    const membersRoleToggle = await within(writeAccessGroup).findByRole("checkbox", {
      name: /Members/i,
    });

    await userEvent.click(membersRoleToggle);
    await userEvent.click(
      screen.getByRole("button", { name: "Save access roles" }),
    );

    await waitFor(() => {
      expect(settingsUpdates).toEqual([
        {
          guildID: "guild-1",
          payload: {
            roles: {
              dashboard_read: ["role-target"],
              dashboard_write: ["role-guard", "role-target"],
            },
          },
        },
      ]);
    });

    expect(
      screen.getByText("Dashboard access roles updated."),
    ).toBeInTheDocument();
  });

  it("keeps Control Panel writes disabled when the selected server is read-only", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState(
      {},
      "",
      appRoutes.dashboardCoreControlPanel("guild-2"),
    );

    render(<App />);

    await screen.findByRole("heading", { name: "Control Panel", level: 1 });

    await waitFor(() => {
      expect(
        screen.getByText(
          "You currently have read-only access to this server. Role changes are disabled.",
        ),
      ).toBeInTheDocument();
    });

    const readAccessGroup = await screen.findByRole("group", {
      name: "Read access roles",
    });
    expect(
      within(readAccessGroup).getByRole("checkbox", { name: /@everyone/i }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Save access roles" }),
    ).toBeDisabled();
  });

  it("includes read-only accessible servers in the server picker", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState(
      {},
      "",
      appRoutes.dashboardCoreControlPanel("guild-1"),
    );

    render(<App />);

    await screen.findByRole("heading", { name: "Control Panel", level: 1 });

    await userEvent.click(await screen.findByRole("button", { name: "Server" }));
    await userEvent.click(
      await screen.findByRole("menuitem", { name: /Server Two/i }),
    );

    await waitFor(() => {
      expect(window.location.pathname).toBe(
        appRoutes.dashboardCoreControlPanel("guild-2"),
      );
    });

    expect(
      await screen.findByText(
        "You currently have read-only access to this server. Role changes are disabled.",
      ),
    ).toBeInTheDocument();
  });

  it(
    "auto-loads Partner Board data again when the selected server changes",
    async () => {
      const { boardCalls, fetchMock } = createFetchMock();
      vi.stubGlobal("fetch", fetchMock);
      window.history.replaceState({}, "", "/dashboard/partner-board/entries");

      render(<App />);

      await screen.findByRole("heading", { name: "Partner Board", level: 1 });
      await waitFor(() => {
        expect(boardCalls).toContain("guild-1");
      });

      await selectServerOption("Server Three");

      await waitFor(() => {
        expect(boardCalls).toContain("guild-3");
      });

      await screen.findByRole("cell", { name: "Server Three" });
    },
    10000,
  );

  it("opens Moderation as a real category workspace and keeps the scope limited to logging, mute role, and moderation routes", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState(
      {},
      "",
      testRoutes.moderation,
    );

    render(<App />);

    await screen.findByRole("heading", { name: "Moderation", level: 1 });
    await screen.findByRole("button", { name: "Disable Automod service" });

    expect(window.location.pathname).toBe(testRoutes.moderation);
    expect(window.location.hash).toBe("");
    expect(screen.getAllByText("Logging only").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Mute role").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Supported actions").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText("ban, massban, kick, mute, timeout, warnings").length,
    ).toBeGreaterThan(0);
    expect(
      screen.getByRole("button", { name: "Disable Automod service" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Rule coverage")).not.toBeInTheDocument();
  });

  it("saves mute role and moderation case route from the dedicated Moderation workspace", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState(
      {},
      "",
      testRoutes.moderation,
    );

    render(<App />);

    await screen.findByRole("heading", { name: "Moderation", level: 1 });
    await screen.findByRole("button", { name: "Configure mute role" });

    await userEvent.click(
      screen.getByRole("button", { name: "Configure mute role" }),
    );

    expect(
      screen.getByRole("dialog", { name: "Configure Mute role" }),
    ).toBeVisible();
    await userEvent.selectOptions(
      screen.getByLabelText("Mute role"),
      "mute-role",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Save mute role" }),
    );

    await waitFor(() => {
      expect(featureUpdates).toEqual([
        {
          guildID: "guild-1",
          featureID: "moderation.mute_role",
          payload: {
            role_id: "mute-role",
          },
        },
      ]);
    });

    await userEvent.click(
      screen.getByRole("button", { name: "Configure Moderation case logging" }),
    );
    expect(
      screen.getByRole("dialog", { name: "Configure Moderation case logging" }),
    ).toBeVisible();
    await userEvent.selectOptions(
      screen.getByLabelText("Destination channel"),
      "mod-cases",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Save destination" }),
    );

    await waitFor(() => {
      expect(featureUpdates).toEqual([
        {
          guildID: "guild-1",
          featureID: "moderation.mute_role",
          payload: {
            role_id: "mute-role",
          },
        },
        {
          guildID: "guild-1",
          featureID: "logging.moderation_case",
          payload: {
            channel_id: "mod-cases",
          },
        },
      ]);
    });

    expect(screen.getAllByText("Muted").length).toBeGreaterThan(0);
    expect(screen.getAllByText("#mod-cases").length).toBeGreaterThan(0);
  });

  it.each(["/dashboard/automations", "/dashboard/activity"])(
    "redirects %s to Home instead of a placeholder page",
    async (path) => {
      const { fetchMock } = createFetchMock();
      vi.stubGlobal("fetch", fetchMock);
      window.history.replaceState({}, "", path);

      render(<App />);

      await screen.findByRole("heading", { name: "Home", level: 1 });
      expect(window.location.pathname).toBe(testRoutes.home);
      expect(window.location.hash).toBe("");
      expect(
        await screen.findByRole("heading", { name: "Core", level: 2 }),
      ).toBeInTheDocument();
    },
  );

  it("keeps Entries, Layout, and Destination on separate routes and removes the placeholder Activity tab", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/partner-board/layout");

    render(<App />);

    await screen.findByRole("heading", { name: "Board text" });
    expect(
      screen.queryByRole("heading", { name: "Manage entries" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Activity" }),
    ).not.toBeInTheDocument();

    await userEvent.click(
      within(
        screen.getByRole("navigation", { name: "Partner Board sections" }),
      ).getByRole("link", { name: "Destination" }),
    );

    await screen.findByRole("heading", {
      name: "Set where the board is published",
    });
    expect(
      screen.queryByRole("heading", { name: "Board text" }),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText("Board message ID")).toBeInTheDocument();
    expect(screen.getByLabelText("Channel ID")).toBeInTheDocument();
    expect(screen.queryByLabelText("Webhook URL")).not.toBeInTheDocument();
  });

  it("uses a drawer for add and edit, and inline confirmation for remove on Partner Board entries", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/partner-board/entries");

    render(<App />);

    await screen.findByRole("heading", { name: "Manage entries" });

    await userEvent.click(screen.getByRole("button", { name: "Add partner" }));
    expect(screen.getByLabelText("Add partner")).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await userEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByLabelText("Edit partner")).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await userEvent.click(screen.getByRole("button", { name: "Remove" }));
    expect(screen.getByRole("button", { name: "Confirm" })).toBeVisible();
  });

  it("redirects the legacy roles-members route to the stable Roles workspace route", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/roles-members");

    render(<App />);

    await screen.findByRole("heading", { name: "Roles", level: 1 });
    expect(window.location.pathname).toBe(testRoutes.rolesAutorole);
  });

  it("opens the dedicated Roles workspace and saves auto role configuration through the drawer", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.rolesAutorole);

    render(<App />);

    await screen.findByRole("heading", { name: "Roles", level: 1 });
    await screen.findByRole("heading", {
      name: "Automatic role assignment",
      level: 2,
    });
    expect(screen.getAllByText("Not configured").length).toBeGreaterThan(0);

    await userEvent.click(
      screen.getByRole("button", { name: "Configure auto role" }),
    );

    expect(
      screen.getByRole("dialog", {
        name: "Configure automatic role assignment",
      }),
    ).toBeVisible();

    await userEvent.selectOptions(
      screen.getByLabelText("Assignment rule"),
      "enabled",
    );
    await userEvent.selectOptions(
      screen.getByLabelText("Target role"),
      "role-target",
    );
    await userEvent.selectOptions(
      screen.getByLabelText("Level role"),
      "role-level",
    );
    await userEvent.selectOptions(
      screen.getByLabelText("Booster role"),
      "role-booster",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Save auto role" }),
    );

    await waitFor(() => {
      expect(featureUpdates).toEqual([
        {
          guildID: "guild-1",
          featureID: "auto_role_assignment",
          payload: {
            config_enabled: true,
            target_role_id: "role-target",
            required_role_ids: ["role-level", "role-booster"],
          },
        },
      ]);
    });

    expect(
      screen.queryByRole("dialog", {
        name: "Configure automatic role assignment",
      }),
    ).not.toBeInTheDocument();
    expect(screen.getAllByText("Members").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Level Five + Boosters").length).toBeGreaterThan(
      0,
    );
  });

  it("uses a member picker for presence watch user instead of a raw user ID field", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.rolesAutorole);

    render(<App />);

    await screen.findByRole("heading", { name: "Roles", level: 1 });
    await screen.findByRole("heading", {
      name: "Automatic role assignment",
      level: 2,
    });
    const advancedControlsSummary =
      screen
        .getAllByText("Advanced controls")
        .find((element) => element.closest("summary") !== null)
        ?.closest("summary") ?? null;
    expect(advancedControlsSummary).not.toBeNull();
    await userEvent.click(advancedControlsSummary!);

    const configureButtons = await screen.findAllByRole("button", {
      name: "Configure",
    });
    await userEvent.click(configureButtons[1]!);

    expect(
      screen.getByRole("dialog", { name: "Configure user presence watch" }),
    ).toBeVisible();
    expect(screen.queryByLabelText("User ID")).not.toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Search members"), "car");

    await waitFor(() => {
      expect(
        screen.getByRole("option", { name: "Carol Gamma (@carol)" }),
      ).toBeInTheDocument();
    });

    await userEvent.selectOptions(
      screen.getByLabelText("Member"),
      "user-carol",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Save user watch" }),
    );

    await waitFor(() => {
      expect(featureUpdates).toEqual([
        {
          guildID: "guild-1",
          featureID: "presence_watch.user",
          payload: {
            user_id: "user-carol",
          },
        },
      ]);
    });

    expect(
      screen.queryByRole("dialog", { name: "Configure user presence watch" }),
    ).not.toBeInTheDocument();
  });

  it("opens Commands on a dedicated route and keeps the primary workspace focused on the two command tasks", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreCommands);

    render(<App />);

    await screen.findByRole("heading", { name: "Commands", level: 1 });
    await screen.findByRole("heading", { name: "Command routing", level: 2 });

    expect(window.location.pathname).toBe(testRoutes.coreCommands);
    expect(
      await screen.findByRole("button", { name: "Configure command channel" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("button", { name: "Configure admin access" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Monitoring")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("columnheader", { name: "Feature" }),
    ).not.toBeInTheDocument();
  });

  it("keeps command setup inside a drawer with pickers first and raw IDs behind Advanced", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreCommands);

    render(<App />);

    await screen.findByRole("heading", { name: "Commands", level: 1 });
    await screen.findByRole("button", { name: "Configure command channel" });

    await userEvent.click(
      screen.getByRole("button", { name: "Configure command channel" }),
    );

    const dialog = screen.getByRole("dialog", { name: "Configure commands" });
    expect(dialog).toBeVisible();
    expect(screen.getByLabelText("Command channel")).toBeVisible();
    const advancedDetails = screen
      .getByText("Advanced", { selector: "summary" })
      .closest("details");
    expect(advancedDetails).not.toBeNull();
    expect(advancedDetails).not.toHaveAttribute("open");

    await userEvent.click(
      screen.getByText("Advanced", { selector: "summary" }),
    );
    expect(advancedDetails).toHaveAttribute("open");
    expect(screen.getByLabelText("Command channel ID fallback")).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(
      screen.queryByRole("dialog", { name: "Configure commands" }),
    ).not.toBeInTheDocument();
  });

  it("opens the dedicated commands workspace and saves a command channel through the drawer", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.home);

    render(<App />);

    await screen.findByRole("heading", { name: "Home", level: 1 });
    await userEvent.click(await screen.findByRole("link", { name: "Open Commands" }));

    await screen.findByRole("heading", { name: "Commands", level: 1 });
    await screen.findByRole("heading", { name: "Command routing", level: 2 });
    expect(window.location.pathname).toBe(testRoutes.coreCommands);
    expect(screen.queryByText("Monitoring")).not.toBeInTheDocument();
    expect(screen.getAllByText("#bot-commands").length).toBeGreaterThan(0);

    await userEvent.click(
      screen.getByRole("button", { name: "Configure command channel" }),
    );

    expect(
      screen.getByRole("dialog", { name: "Configure commands" }),
    ).toBeVisible();
    expect(screen.getByLabelText("Command channel")).toHaveValue(
      "bot-commands",
    );

    await userEvent.selectOptions(
      screen.getByLabelText("Command channel"),
      "ops-commands",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Save command channel" }),
    );

    await waitFor(() => {
      expect(featureUpdates).toEqual([
        {
          guildID: "guild-1",
          featureID: "services.commands",
          payload: {
            channel_id: "ops-commands",
          },
        },
      ]);
    });

    expect(
      screen.queryByRole("dialog", { name: "Configure commands" }),
    ).not.toBeInTheDocument();
    expect(screen.getAllByText("#ops-commands").length).toBeGreaterThan(0);
  });

  it("saves admin command access through the dedicated commands drawer", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreCommands);

    render(<App />);

    await screen.findByRole("heading", { name: "Commands", level: 1 });
    await screen.findByRole("heading", { name: "Command routing", level: 2 });
    await screen.findByRole("button", { name: "Configure admin access" });

    await userEvent.click(
      screen.getByRole("button", { name: "Configure admin access" }),
    );

    expect(
      screen.getByRole("dialog", { name: "Configure admin commands" }),
    ).toBeVisible();
    expect(screen.getByRole("checkbox", { name: /Moderators/i })).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: /Members/i }),
    ).not.toBeChecked();

    await userEvent.click(screen.getByRole("checkbox", { name: /Members/i }));
    await userEvent.click(
      screen.getByRole("button", { name: "Save admin access" }),
    );

    await waitFor(() => {
      expect(featureUpdates).toEqual([
        {
          guildID: "guild-1",
          featureID: "services.admin_commands",
          payload: {
            allowed_role_ids: ["role-guard", "role-target"],
          },
        },
      ]);
    });

    expect(
      screen.queryByRole("dialog", { name: "Configure admin commands" }),
    ).not.toBeInTheDocument();
    expect(screen.getAllByText("2 roles").length).toBeGreaterThan(0);
  });

  it("opens the dedicated logging workspace and saves a destination through the drawer", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.moderationLogging);

    render(<App />);

    await screen.findByRole("heading", { name: "Logging", level: 1 });
    expect(await screen.findByText("#user-log-channel")).toBeInTheDocument();
    expect(await screen.findByText("Not configured")).toBeInTheDocument();

    const configureButtons = await screen.findAllByRole("button", {
      name: "Configure",
    });
    await userEvent.click(configureButtons[0]!);

    expect(
      screen.getByRole("dialog", { name: "Configure Avatar logging" }),
    ).toBeVisible();
    expect(screen.getByLabelText("Destination channel")).toHaveValue(
      "user-log-channel",
    );
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await userEvent.click(
      screen.getAllByRole("button", { name: "Configure" })[1]!,
    );
    expect(
      screen.getByRole("dialog", { name: "Configure Member join logging" }),
    ).toBeVisible();
    expect(screen.getByLabelText("Destination channel")).toHaveValue("");

    await userEvent.selectOptions(
      screen.getByLabelText("Destination channel"),
      "join-channel",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Save destination" }),
    );

    await waitFor(() => {
      expect(featureUpdates).toEqual([
        {
          guildID: "guild-1",
          featureID: "logging.member_join",
          payload: {
            channel_id: "join-channel",
          },
        },
      ]);
    });

    expect(
      screen.queryByRole("dialog", { name: "Configure Member join logging" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("#join-channel")).toBeInTheDocument();
  });

  it("redirects the legacy maintenance route to Home", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/maintenance");

    render(<App />);

    await screen.findByRole("heading", { name: "Home", level: 1 });
    expect(window.location.pathname).toBe(testRoutes.home);
  });

  it("opens the dedicated Stats workspace and replaces the generic feature table with a schedule-focused workspace", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreStats);

    render(<App />);

    await screen.findByRole("heading", { name: "Stats", level: 1 });
    await screen.findByRole("heading", { name: "Stats updates", level: 2 });

    expect(window.location.pathname).toBe(testRoutes.coreStats);
    expect(
      screen.getByRole("button", { name: "Configure stats schedule" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("2 channels").length).toBeGreaterThan(0);
    expect(screen.getByText("Total members")).toBeInTheDocument();
    expect(screen.getByText("Bot count")).toBeInTheDocument();
    expect(
      screen.queryByRole("columnheader", { name: "Feature" }),
    ).not.toBeInTheDocument();
  });

  it("saves stats activation and interval from the dedicated Stats workspace", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreStats);

    render(<App />);

    await screen.findByRole("heading", { name: "Stats", level: 1 });
    await screen.findByRole("button", { name: "Configure stats schedule" });

    await userEvent.click(
      screen.getByRole("button", { name: "Configure stats schedule" }),
    );

    expect(
      screen.getByRole("dialog", { name: "Configure Stats channels" }),
    ).toBeVisible();

    await userEvent.selectOptions(
      screen.getByLabelText("Update rule"),
      "enabled",
    );
    await userEvent.clear(screen.getByLabelText("Update interval (minutes)"));
    await userEvent.type(
      screen.getByLabelText("Update interval (minutes)"),
      "45",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Save stats settings" }),
    );

    await waitFor(() => {
      expect(featureUpdates).toEqual([
        {
          guildID: "guild-1",
          featureID: "stats_channels",
          payload: {
            config_enabled: true,
            update_interval_mins: 45,
          },
        },
      ]);
    });

    expect(
      screen.queryByRole("dialog", { name: "Configure Stats channels" }),
    ).not.toBeInTheDocument();
    expect(screen.getAllByText("45 minutes").length).toBeGreaterThan(0);

    await userEvent.click(
      screen.getByRole("button", { name: "Disable stats module" }),
    );

    await waitFor(() => {
      expect(featureUpdates).toEqual([
        {
          guildID: "guild-1",
          featureID: "stats_channels",
          payload: {
            config_enabled: true,
            update_interval_mins: 45,
          },
        },
        {
          guildID: "guild-1",
          featureID: "stats_channels",
          payload: {
            enabled: false,
          },
        },
      ]);
    });

    expect(
      await screen.findByRole("button", { name: "Enable stats module" }),
    ).toBeInTheDocument();
  });

  it("edits Partner Board delivery inline without handing off to Settings", async () => {
    const { fetchMock, targetUpdates } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/partner-board/delivery");

    render(<App />);

    await screen.findByRole("heading", {
      name: "Set where the board is published",
    });
    await userEvent.selectOptions(
      screen.getByLabelText("Posting method"),
      "webhook_message",
    );
    await userEvent.clear(screen.getByLabelText("Board message ID"));
    await userEvent.type(
      screen.getByLabelText("Board message ID"),
      "999999999999999999",
    );
    await userEvent.clear(screen.getByLabelText("Webhook URL"));
    await userEvent.type(
      screen.getByLabelText("Webhook URL"),
      "https://discord.com/api/webhooks/new-target",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Save destination" }),
    );

    await waitFor(() => {
      expect(targetUpdates).toEqual([
        {
          guildID: "guild-1",
          payload: {
            type: "webhook_message",
            message_id: "999999999999999999",
            webhook_url: "https://discord.com/api/webhooks/new-target",
          },
        },
      ]);
    });

    expect(screen.getAllByText("Posting destination updated.").length).toBeGreaterThan(0);
  });
});
