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
import { clearGuildRolesSettingsCache } from "./features/control-panel/useGuildRolesSettings";
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

function expectStandardDashboardBlacklistAbsent() {
  expect(screen.queryByText("Configured here")).not.toBeInTheDocument();
  expect(screen.queryByText("Using default")).not.toBeInTheDocument();
  expect(screen.queryByText("Enabled for this server")).not.toBeInTheDocument();
  expect(screen.queryByText(/^Applied from$/)).not.toBeInTheDocument();
  expect(screen.queryByText(/^Override$/)).not.toBeInTheDocument();
  expect(screen.queryByText(/local overrides/i)).not.toBeInTheDocument();
  expect(screen.queryByText(/routes configured/i)).not.toBeInTheDocument();
  expect(screen.queryByText(/Server:/)).not.toBeInTheDocument();
  expect(screen.queryByText(/Origin:/)).not.toBeInTheDocument();
  expect(screen.queryByLabelText("Command channel ID fallback")).not.toBeInTheDocument();
  expect(
    screen.queryByLabelText("Destination channel ID fallback"),
  ).not.toBeInTheDocument();
  expect(screen.queryByLabelText("Mute role ID fallback")).not.toBeInTheDocument();
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

function getURLPath(input: RequestInfo | URL) {
  const url = typeof input === "string" ? input : input.toString();
  return new URL(url, "https://dashboard.test").pathname;
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
      botRouting: {
        bot_instance_id: string;
        available_bot_instance_ids: string[];
        domain_bot_instance_ids: Record<string, string>;
        editable_domains: string[];
      };
    }
  > = {
    "guild-1": {
      roles: {
        allowed: ["role-guard"],
        dashboard_read: ["role-target"],
        dashboard_write: ["role-guard"],
      },
      botRouting: {
        bot_instance_id: "alice",
        available_bot_instance_ids: ["alice", "companion"],
        domain_bot_instance_ids: {
          qotd: "companion",
        },
        editable_domains: ["qotd"],
      },
    },
    "guild-2": {
      roles: {
        allowed: [],
        dashboard_read: [],
        dashboard_write: [],
      },
      botRouting: {
        bot_instance_id: "alice",
        available_bot_instance_ids: ["alice"],
        domain_bot_instance_ids: {},
        editable_domains: ["qotd"],
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
        id: "logging.clean_action",
        category: "logging",
        label: "Clean action logging",
        description: "Clean actions",
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
          runtime_toggle_path: "disable_clean_log",
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
      botRouting: {
        bot_instance_id: "alice",
        available_bot_instance_ids: ["alice"],
        domain_bot_instance_ids: {},
        editable_domains: ["qotd"],
      },
    };

    return {
      status: "ok",
      workspace: {
        scope: "guild",
        guild_id: guildID,
        bot_instance_id: settings.botRouting.bot_instance_id,
        available_bot_instance_ids: [...settings.botRouting.available_bot_instance_ids],
        sections: {
          bot_routing: {
            bot_instance_id: settings.botRouting.bot_instance_id,
            available_bot_instance_ids: [...settings.botRouting.available_bot_instance_ids],
            domain_bot_instance_ids: { ...settings.botRouting.domain_bot_instance_ids },
            editable_domains: [...settings.botRouting.editable_domains],
          },
          roles: {
            allowed: [...settings.roles.allowed],
            dashboard_read: [...settings.roles.dashboard_read],
            dashboard_write: [...settings.roles.dashboard_write],
          },
        },
      },
    };
  }

  function normalizeStringRecord(input: unknown) {
    if (typeof input !== "object" || input === null) {
      return {} as Record<string, string>;
    }

    const normalized: Record<string, string> = {};
    for (const [key, value] of Object.entries(input as Record<string, unknown>)) {
      if (typeof value !== "string") {
        continue;
      }
      normalized[key] = value;
    }

    return normalized;
  }

  const fetchMock = vi.fn(
    async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      const pathname = getURLPath(input);

      if (pathname === "/auth/me") {
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

      if (pathname === "/auth/guilds/access") {
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

      if (pathname === "/auth/guilds/manageable") {
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
          const currentSettings = guildSettingsByGuild[guildID] ?? {
            roles: {
              allowed: [],
              dashboard_read: [],
              dashboard_write: [],
            },
            botRouting: {
              bot_instance_id: "alice",
              available_bot_instance_ids: ["alice"],
              domain_bot_instance_ids: {},
              editable_domains: ["qotd"],
            },
          };
          const rolesPayload =
            typeof payload.roles === "object" && payload.roles !== null
              ? (payload.roles as Record<string, unknown>)
              : {};
          const botRoutingPayload =
            typeof payload.bot_routing === "object" && payload.bot_routing !== null
              ? (payload.bot_routing as Record<string, unknown>)
              : {};
          guildSettingsByGuild[guildID] = {
            roles: {
              allowed: currentSettings.roles.allowed,
              dashboard_read: Array.isArray(rolesPayload.dashboard_read)
                ? rolesPayload.dashboard_read.filter(
                    (value): value is string => typeof value === "string",
                  )
                : currentSettings.roles.dashboard_read,
              dashboard_write: Array.isArray(rolesPayload.dashboard_write)
                ? rolesPayload.dashboard_write.filter(
                    (value): value is string => typeof value === "string",
                  )
                : currentSettings.roles.dashboard_write,
            },
            botRouting: {
              bot_instance_id:
                typeof botRoutingPayload.bot_instance_id === "string"
                  ? botRoutingPayload.bot_instance_id.trim()
                  : currentSettings.botRouting.bot_instance_id,
              available_bot_instance_ids: [...currentSettings.botRouting.available_bot_instance_ids],
              domain_bot_instance_ids:
                Object.prototype.hasOwnProperty.call(
                  botRoutingPayload,
                  "domain_bot_instance_ids",
                )
                  ? normalizeStringRecord(botRoutingPayload.domain_bot_instance_ids)
                  : { ...currentSettings.botRouting.domain_bot_instance_ids },
              editable_domains: [...currentSettings.botRouting.editable_domains],
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

function createReadOnlySelectedGuildFetchMock() {
  const base = createFetchMock();
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const pathname = getURLPath(input);

    if (pathname === "/auth/guilds/access") {
      return jsonResponse({
        status: "ok",
        count: 3,
        guilds: [
          {
            id: "guild-1",
            name: "Server One",
            owner: false,
            permissions: 32,
            access_level: "read",
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

    if (pathname === "/auth/guilds/manageable") {
      return jsonResponse({
        status: "ok",
        count: 1,
        guilds: [
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

    return base.fetchMock(input, init);
  });

  return {
    ...base,
    fetchMock,
  };
}

describe("dashboard routing and workspace", () => {
  beforeEach(() => {
    window.history.replaceState({}, "", appRoutes.legacyControlPanel);
  });

  afterEach(() => {
    cleanup();
    clearGuildRolesSettingsCache();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    window.history.replaceState({}, "", "/");
  });

  it("renders the lean shell, preserves the legacy control-panel redirect, and keeps global context in the top bar", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", appRoutes.legacyControlPanel);

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
    const engagementButton = within(sidebarNav).getByRole("button", {
      name: "Engagement",
    });
    const rolesButton = within(sidebarNav).getByRole("button", { name: "Roles" });
    expect(coreButton).toHaveAttribute("aria-expanded", "true");
    expect(moderationButton).toHaveAttribute("aria-expanded", "false");
    expect(engagementButton).toHaveAttribute("aria-expanded", "false");
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
    await userEvent.click(engagementButton);

    expect(moderationButton).toHaveAttribute("aria-expanded", "false");
    expect(engagementButton).toHaveAttribute("aria-expanded", "true");
    expect(
      within(sidebarNav).getByRole("link", { name: "Partner Board" }),
    ).toBeInTheDocument();
    expect(within(sidebarNav).getByRole("link", { name: "QOTD" })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Reconnect" }),
    ).not.toBeInTheDocument();
  }, 10000);

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
      within(sidebarNav).getByRole("button", { name: "Engagement" }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(
      within(sidebarNav).getByRole("button", { name: "Roles" }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(
      within(sidebarNav).queryByRole("link", { name: "Control Panel" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Core", level: 2 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Moderation", level: 2 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Engagement", level: 2 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Roles", level: 2 })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Control Panel" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Stats" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Commands" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Logging" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open Partner Board" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Open QOTD" })).toBeInTheDocument();
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
  }, 10000);

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
    window.history.replaceState({}, "", appRoutes.legacyControlPanel);

    render(<App />);

    await screen.findByRole("heading", { name: "Control Panel", level: 1 });
    await userEvent.click(
      await screen.findByRole("button", { name: /write access roles/i }),
    );
    const writeAccessGroup = await screen.findByRole("group", {
      name: "Write access roles",
    });
    const membersRoleToggle = await within(writeAccessGroup).findByRole("checkbox", {
      name: /Members/i,
    });

    await userEvent.click(membersRoleToggle);
    await userEvent.click(
      screen.getAllByRole("button", { name: "Save changes" }).at(-1)!,
    );

    await waitFor(() => {
      expect(settingsUpdates).toEqual([
        {
          guildID: "guild-1",
          payload: {
            bot_routing: {
              bot_instance_id: "alice",
              domain_bot_instance_ids: {
                qotd: "companion",
              },
            },
            roles: {
              dashboard_read: ["role-target"],
              dashboard_write: ["role-guard", "role-target"],
            },
          },
        },
      ]);
    });

    expect(
      screen.queryByText("Dashboard access roles updated."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryAllByRole("button", { name: "Save changes" }),
    ).toHaveLength(0);
  });

  it("saves bot routing changes from the dedicated Control Panel page", async () => {
    const { fetchMock, settingsUpdates } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", appRoutes.legacyControlPanel);

    render(<App />);

    await screen.findByRole("heading", { name: "Control Panel", level: 1 });
    const defaultBotSelect = await screen.findByRole("combobox", {
      name: "Default bot instance",
    });
    await waitFor(() => {
      expect(within(defaultBotSelect).getByRole("option", { name: "Companion" })).toBeInTheDocument();
    });

    await userEvent.selectOptions(
      defaultBotSelect,
      "companion",
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "QOTD domain" }),
      "",
    );
    await userEvent.click(
      screen.getAllByRole("button", { name: "Save changes" }).at(-1)!,
    );

    await waitFor(() => {
      expect(settingsUpdates).toEqual([
        {
          guildID: "guild-1",
          payload: {
            bot_routing: {
              bot_instance_id: "companion",
              domain_bot_instance_ids: {},
            },
            roles: {
              dashboard_read: ["role-target"],
              dashboard_write: ["role-guard"],
            },
          },
        },
      ]);
    });
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

    const readAccessTrigger = screen.getByRole("button", {
      name: /read access roles/i,
    });
    expect(readAccessTrigger).toBeDisabled();
    expect(
      screen.getByRole("button", { name: /write access roles/i }),
    ).toBeDisabled();
    expect(
      screen.queryAllByRole("button", { name: "Save changes" }),
    ).toHaveLength(0);
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
    await screen.findByRole("checkbox", { name: "Automod service" });

    expect(window.location.pathname).toBe(testRoutes.moderation);
    expect(window.location.hash).toBe("");
    expect(
      screen.getByRole("group", { name: "Mute role" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(
        "Moderation command toggles, mute-role setup, and moderation event routes for staff workflows.",
      ),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Moderation routes", level: 2 }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", { name: "Automod service" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Supported actions")).not.toBeInTheDocument();
    expect(
      screen.queryByText("ban, massban, kick, mute, timeout, warnings"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Logging only")).not.toBeInTheDocument();
    expect(screen.queryByText("Current signal")).not.toBeInTheDocument();
    expect(screen.queryByText("Use inherited")).not.toBeInTheDocument();
    expect(screen.queryByText("Use default")).not.toBeInTheDocument();
    expect(screen.queryByText("How this page works")).not.toBeInTheDocument();
    expect(screen.queryByText("Moderation controls")).not.toBeInTheDocument();
    expect(screen.queryByText("Rule coverage")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Discord native AutoMod listener used for logging."),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Role applied by the mute command.")).not.toBeInTheDocument();
    expect(screen.queryByText("AutoMod executions")).not.toBeInTheDocument();
    expect(screen.queryByText("Moderation cases")).not.toBeInTheDocument();
    expect(
      screen.getByText("Choose the role that should be applied by the mute command."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Choose a destination channel for this logging route."),
    ).toBeInTheDocument();
    expect(
      screen
        .getByRole("heading", { name: "Moderation", level: 1 })
        .closest(".flat-page-workspace"),
    ).not.toBeNull();
  });

  it("saves mute role and moderation case route inline from the dedicated Moderation workspace", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState(
      {},
      "",
      testRoutes.moderation,
    );

    render(<App />);

    await screen.findByRole("heading", { name: "Moderation", level: 1 });
    const muteRoleSection = screen.getByRole("group", {
      name: "Mute role",
    });
    await userEvent.selectOptions(
      within(muteRoleSection).getByLabelText("Role"),
      "mute-role",
    );
    await userEvent.click(
      within(muteRoleSection).getByRole("button", { name: "Save changes" }),
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

    const moderationCaseSection = screen.getByRole("group", {
      name: "Moderation case logging",
    });
    await userEvent.selectOptions(
      within(moderationCaseSection).getByLabelText("Channel"),
      "mod-cases",
    );
    await userEvent.click(
      within(moderationCaseSection).getByRole("button", {
        name: "Save changes",
      }),
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
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("toggles automod directly from the moderation workspace", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.moderation);

    render(<App />);

    await screen.findByRole("heading", { name: "Moderation", level: 1 });

    const automodToggle = screen.getByRole("checkbox", {
      name: "Automod service",
    });
    expect(automodToggle).toBeChecked();

    await userEvent.click(automodToggle);

    await waitFor(() => {
      expect(featureUpdates[0]).toEqual({
        guildID: "guild-1",
        featureID: "services.automod",
        payload: {
          enabled: false,
        },
      });
    });
  });

  it("requires a mute role selection before re-enabling the feature", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.moderation);

    render(<App />);

    await screen.findByRole("heading", { name: "Moderation", level: 1 });

    const muteSection = screen.getByRole("group", { name: "Mute role" });

    const muteToggle = within(muteSection).getByRole("checkbox", {
      name: "Mute role",
    });
    expect(
      within(muteSection).getByLabelText("Role"),
    ).toBeInTheDocument();

    await userEvent.click(muteToggle);

    await waitFor(() => {
      expect(featureUpdates[0]).toEqual({
        guildID: "guild-1",
        featureID: "moderation.mute_role",
        payload: {
          enabled: false,
        },
      });
    });
    await waitFor(() => {
      expect(
        within(muteSection).getByLabelText("Role"),
      ).toBeInTheDocument();
    });
    expect(
      within(muteSection).getByRole("checkbox", { name: "Mute role" }),
    ).toBeDisabled();

    await userEvent.selectOptions(
      within(muteSection).getByLabelText("Role"),
      "mute-role",
    );

    await userEvent.click(
      within(muteSection).getByRole("checkbox", {
        name: "Mute role",
      }),
    );

    await waitFor(() => {
      expect(featureUpdates[1]).toEqual({
        guildID: "guild-1",
        featureID: "moderation.mute_role",
        payload: {
          role_id: "mute-role",
          enabled: true,
        },
      });
    });
    await waitFor(() => {
      expect(
        within(muteSection).getByLabelText("Role"),
      ).toBeInTheDocument();
    });
  });

  it("resets inline moderation drafts", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.moderation);

    render(<App />);

    await screen.findByRole("heading", { name: "Moderation", level: 1 });

    function getAutomodActionSection() {
      return screen.getByRole("group", { name: "Automod action logging" });
    }

    const routeChannelSelect =
      within(getAutomodActionSection()).getByLabelText("Channel");
    await userEvent.selectOptions(routeChannelSelect, "mod-cases");
    expect(routeChannelSelect).toHaveValue("mod-cases");
    await waitFor(() => {
      expect(
        within(getAutomodActionSection()).getByRole("button", { name: "Reset" }),
      ).toBeInTheDocument();
    });
    await userEvent.click(
      within(getAutomodActionSection()).getByRole("button", { name: "Reset" }),
    );
    expect(routeChannelSelect).toHaveValue("mod-actions");
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

    await userEvent.click(document.querySelector(".drawer-backdrop")!);

    await userEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByLabelText("Edit partner")).toBeVisible();

    await userEvent.click(document.querySelector(".drawer-backdrop")!);
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

  it("opens the dedicated Roles workspace and saves auto role configuration inline", async () => {
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
    const autoRoleSection = screen
      .getByRole("heading", {
        name: "Automatic role assignment",
        level: 2,
      })
      .closest("section");
    expect(autoRoleSection).not.toBeNull();

    await userEvent.selectOptions(
      within(autoRoleSection!).getByLabelText("Assignment rule"),
      "enabled",
    );
    await userEvent.selectOptions(
      within(autoRoleSection!).getByLabelText("Target role"),
      "role-target",
    );
    await userEvent.selectOptions(
      within(autoRoleSection!).getByLabelText("Level role"),
      "role-level",
    );
    await userEvent.selectOptions(
      within(autoRoleSection!).getByLabelText("Booster role"),
      "role-booster",
    );
    await userEvent.click(
      within(autoRoleSection!).getByRole("button", { name: "Save changes" }),
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

    expect(screen.getAllByText("Members").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Level Five + Boosters").length).toBeGreaterThan(
      0,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("uses a member picker for presence watch user inline instead of a raw user ID field", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.rolesAutorole);

    render(<App />);

    await screen.findByRole("heading", { name: "Roles", level: 1 });
    await screen.findByRole("heading", {
      name: "Automatic role assignment",
      level: 2,
    });
    const presenceWatchSection = screen
      .getByRole("heading", { name: "Presence watch (user)", level: 2 })
      .closest("section");
    expect(presenceWatchSection).not.toBeNull();
    expect(screen.queryByLabelText("User ID")).not.toBeInTheDocument();

    await userEvent.type(
      within(presenceWatchSection!).getByLabelText("Search members"),
      "car",
    );

    await waitFor(() => {
      expect(
        within(presenceWatchSection!).getByRole("option", {
          name: "Carol Gamma (@carol)",
        }),
      ).toBeInTheDocument();
    });

    await userEvent.selectOptions(
      within(presenceWatchSection!).getByLabelText("Member"),
      "user-carol",
    );
    await userEvent.click(
      within(presenceWatchSection!).getByRole("button", {
        name: "Save changes",
      }),
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

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("opens Commands on a dedicated route and keeps the primary workspace focused on the two command tasks", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreCommands);

    render(<App />);

    await screen.findByRole("heading", { name: "Commands", level: 1 });
    await screen.findByRole("heading", { name: "Command routing", level: 2 });

    expect(window.location.pathname).toBe(testRoutes.coreCommands);
    expect(await screen.findByLabelText("Command channel")).toBeInTheDocument();
    expect(
      await screen.findByRole("button", { name: /Allowed roles/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /refresh commands/i }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Current command setup")).not.toBeInTheDocument();
    expect(screen.queryByText("How this page works")).not.toBeInTheDocument();
    expect(screen.queryByText("Monitoring")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("columnheader", { name: "Feature" }),
    ).not.toBeInTheDocument();
  });

  it("keeps command setup inline without exposing raw ID fallbacks in standard mode", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreCommands);

    render(<App />);

    await screen.findByRole("heading", { name: "Commands", level: 1 });
    expect(screen.getByLabelText("Command channel")).toBeVisible();
    expect(screen.queryByText("Advanced", { selector: "summary" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Command channel ID fallback")).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("opens the dedicated commands workspace and saves a command channel inline", async () => {
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
    const commandSection = screen
      .getByRole("heading", { name: "Command routing", level: 2 })
      .closest("section");
    expect(commandSection).not.toBeNull();
    expect(within(commandSection!).getByLabelText("Command channel")).toHaveValue(
      "bot-commands",
    );

    await userEvent.selectOptions(
      within(commandSection!).getByLabelText("Command channel"),
      "ops-commands",
    );
    await userEvent.click(
      within(commandSection!).getByRole("button", { name: "Save changes" }),
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

    expect(screen.getAllByText("#ops-commands").length).toBeGreaterThan(0);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("saves admin command access inline from the dedicated commands workspace", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreCommands);

    render(<App />);

    await screen.findByRole("heading", { name: "Commands", level: 1 });
    await screen.findByRole("heading", { name: "Command routing", level: 2 });
    const adminSection = screen
      .getByRole("heading", { name: "Admin command access", level: 2 })
      .closest("section");
    expect(adminSection).not.toBeNull();
    await userEvent.click(
      within(adminSection!).getByRole("button", { name: /allowed roles/i }),
    );
    expect(
      within(adminSection!).getByRole("checkbox", { name: /Moderators/i }),
    ).toBeChecked();
    expect(
      within(adminSection!).getByRole("checkbox", { name: /Members/i }),
    ).not.toBeChecked();

    await userEvent.click(
      within(adminSection!).getByRole("checkbox", { name: /Members/i }),
    );
    await userEvent.click(
      within(adminSection!).getByRole("button", { name: "Save changes" }),
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

    expect(screen.getAllByText("2 roles").length).toBeGreaterThan(0);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("keeps command workspace affordances disabled for read-only servers", async () => {
    const { featureUpdates, fetchMock } = createReadOnlySelectedGuildFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreCommands);

    render(<App />);

    await screen.findByRole("heading", { name: "Commands", level: 1 });

    const commandChannelField = screen.getByLabelText("Command channel");
    const allowedRolesButton = screen.getByRole("button", {
      name: /Allowed roles/i,
    });
    const disableCommandsButton = screen.getByRole("button", {
      name: "Disable commands",
    });
    const disableAdminCommandsButton = screen.getByRole("button", {
      name: "Disable admin commands",
    });

    expect(commandChannelField).toBeDisabled();
    expect(allowedRolesButton).toBeDisabled();
    expect(disableCommandsButton).toBeDisabled();
    expect(disableAdminCommandsButton).toBeDisabled();

    await userEvent.click(allowedRolesButton);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(featureUpdates).toEqual([]);
  });

  it("opens the dedicated logging workspace and saves a destination inline", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.moderationLogging);

    render(<App />);

    await screen.findByRole("heading", { name: "Logging", level: 1 });
    await screen.findByRole("heading", { name: "Logging routes", level: 2 });
    expect(window.location.pathname).toBe(testRoutes.moderationLogging);
    expect(screen.queryByText("Feature area")).not.toBeInTheDocument();
    expect(screen.queryByText("Manage logging routes")).not.toBeInTheDocument();
    expect(screen.queryByText("Current blocker")).not.toBeInTheDocument();
    expect(screen.queryByText("Current signal")).not.toBeInTheDocument();
    expect(screen.queryByText("Destination rule")).not.toBeInTheDocument();
    expectStandardDashboardBlacklistAbsent();

    const avatarSection = screen.getByRole("group", {
      name: "Avatar logging",
    });
    expect(
      within(avatarSection).getAllByText("#user-log-channel").length,
    ).toBeGreaterThan(0);
    expect(within(avatarSection).getByLabelText("Destination channel")).toHaveValue(
      "user-log-channel",
    );
    const memberJoinSection = screen.getByRole("group", {
      name: "Member join logging",
    });
    expect(
      within(memberJoinSection).getAllByText("No destination channel").length,
    ).toBeGreaterThan(0);
    expect(
      within(memberJoinSection).getByLabelText("Destination channel"),
    ).toHaveValue("");
    const cleanActionSection = screen.getByRole("group", {
      name: "Clean action logging",
    });
    expect(
      within(cleanActionSection).getByLabelText("Destination channel"),
    ).toHaveValue("");

    await userEvent.selectOptions(
      within(memberJoinSection).getByLabelText("Destination channel"),
      "join-channel",
    );
    await userEvent.click(
      within(memberJoinSection).getByRole("button", { name: "Save changes" }),
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

    await userEvent.selectOptions(
      within(cleanActionSection).getByLabelText("Destination channel"),
      "mod-actions",
    );
    await userEvent.click(
      within(cleanActionSection).getByRole("button", { name: "Save changes" }),
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
        {
          guildID: "guild-1",
          featureID: "logging.clean_action",
          payload: {
            channel_id: "mod-actions",
          },
        },
      ]);
    });

    expect(
      within(memberJoinSection).getAllByText("#join-channel").length,
    ).toBeGreaterThan(0);
    expect(
      within(cleanActionSection).getAllByText("#mod-actions").length,
    ).toBeGreaterThan(0);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("keeps logging workspace affordances disabled for read-only servers", async () => {
    const { featureUpdates, fetchMock } = createReadOnlySelectedGuildFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.moderationLogging);

    render(<App />);

    await screen.findByRole("heading", { name: "Logging", level: 1 });
    const avatarSection = screen.getByRole("group", {
      name: "Avatar logging",
    });
    const memberJoinSection = screen.getByRole("group", {
      name: "Member join logging",
    });

    expect(
      within(avatarSection).getByLabelText("Destination channel"),
    ).toBeDisabled();
    expect(
      within(memberJoinSection).getByLabelText("Destination channel"),
    ).toBeDisabled();
    expect(
      screen.getByRole("checkbox", { name: "Avatar logging" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("checkbox", { name: "Member join logging" }),
    ).toBeDisabled();
    expect(
      screen.queryByRole("button", { name: "Disable Avatar logging" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Disable Member join logging" }),
    ).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("checkbox", { name: "Avatar logging" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(featureUpdates).toEqual([]);
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
    expect(screen.getByLabelText("Update rule")).toBeInTheDocument();
    expect(screen.getByLabelText("Update interval (minutes)")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Stats channel inventory", level: 2 }),
    ).toBeInTheDocument();
    expect(screen.getByText("Total members")).toBeInTheDocument();
    expect(screen.getByText("Bot count")).toBeInTheDocument();
    expect(
      screen.queryByRole("columnheader", { name: "Feature" }),
    ).not.toBeInTheDocument();
  });

  it.each([
    { route: testRoutes.coreControlPanel, heading: "Control Panel" },
    { route: testRoutes.coreCommands, heading: "Commands" },
    { route: testRoutes.moderation, heading: "Moderation" },
    { route: testRoutes.moderationLogging, heading: "Logging" },
    { route: testRoutes.rolesAutorole, heading: "Roles" },
    { route: testRoutes.coreStats, heading: "Stats" },
  ])(
    "keeps blacklisted dashboard metadata out of $heading",
    async ({ route, heading }) => {
      const { fetchMock } = createFetchMock();
      vi.stubGlobal("fetch", fetchMock);
      window.history.replaceState({}, "", route);

      render(<App />);

      await screen.findByRole("heading", { name: heading, level: 1 });

      expectStandardDashboardBlacklistAbsent();
    },
  );

  it("saves stats activation and interval inline from the dedicated Stats workspace", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreStats);

    render(<App />);

    await screen.findByRole("heading", { name: "Stats", level: 1 });
    const statsSection = screen
      .getByRole("heading", { name: "Stats updates", level: 2 })
      .closest("section");
    expect(statsSection).not.toBeNull();

    await userEvent.selectOptions(
      within(statsSection!).getByLabelText("Update rule"),
      "enabled",
    );
    await userEvent.clear(
      within(statsSection!).getByLabelText("Update interval (minutes)"),
    );
    await userEvent.type(
      within(statsSection!).getByLabelText("Update interval (minutes)"),
      "45",
    );
    await userEvent.click(
      within(statsSection!).getByRole("button", { name: "Save changes" }),
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

    expect(screen.getAllByText("45 minutes").length).toBeGreaterThan(0);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

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

  it("keeps stats workspace affordances disabled for read-only servers", async () => {
    const { featureUpdates, fetchMock } = createReadOnlySelectedGuildFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", testRoutes.coreStats);

    render(<App />);

    await screen.findByRole("heading", { name: "Stats", level: 1 });

    const updateRuleField = screen.getByLabelText("Update rule");
    const updateIntervalField = screen.getByLabelText("Update interval (minutes)");
    const statsToggleButton = screen.getByRole("button", {
      name: /stats module/i,
    });

    expect(updateRuleField).toBeDisabled();
    expect(updateIntervalField).toBeDisabled();
    expect(statsToggleButton).toBeDisabled();

    expect(
      screen.queryByRole("dialog"),
    ).not.toBeInTheDocument();
    expect(featureUpdates).toEqual([]);
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
      screen.getAllByRole("button", { name: "Save changes" }).at(-1)!,
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
