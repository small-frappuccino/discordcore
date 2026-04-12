import type { FeatureRecord, GuildChannelOption } from "../../api/control";
import { formatGuildChannelValue } from "./discordEntities";

const defaultStatsIntervalMins = 30;

export interface StatsChannelDetail {
  channelId: string;
  label: string;
  nameTemplate: string;
  memberType: string;
  roleId: string;
}

export interface StatsFeatureDetails {
  configEnabled: boolean;
  updateIntervalMins: number;
  configuredChannelCount: number;
  channels: StatsChannelDetail[];
}

export function getStatsFeatureDetails(
  feature: FeatureRecord,
): StatsFeatureDetails {
  const details = feature.details ?? {};
  const channels = readStatsChannelList(details.channels);
  const reportedCount = readNumber(details.configured_channel_count);

  return {
    configEnabled: readBoolean(details.config_enabled),
    updateIntervalMins: normalizeStatsIntervalMins(
      readNumber(details.update_interval_mins),
    ),
    configuredChannelCount: Math.max(reportedCount, channels.length),
    channels,
  };
}

export function summarizeStatsSignal(feature: FeatureRecord) {
  const details = getStatsFeatureDetails(feature);
  const firstBlocker = feature.blockers?.[0]?.message?.trim() ?? "";

  if (!feature.effective_enabled) {
    return "Stats channel updates are currently off for the selected server.";
  }

  if (firstBlocker !== "") {
    return firstBlocker;
  }

  if (!details.configEnabled) {
    return "Stats channel config is disabled.";
  }

  if (details.configuredChannelCount === 0) {
    return "Stats channels need at least one configured target.";
  }

  return "Stats channel updates are ready for the configured channel list.";
}

export function formatStatsConfigValue(configEnabled: boolean) {
  return configEnabled ? "Enabled" : "Disabled";
}

export function formatStatsIntervalValue(updateIntervalMins: number) {
  if (updateIntervalMins === 1) {
    return "1 minute";
  }

  return `${updateIntervalMins} minutes`;
}

export function formatStatsChannelCountValue(configuredChannelCount: number) {
  if (configuredChannelCount === 0) {
    return "No channels";
  }
  if (configuredChannelCount === 1) {
    return "1 channel";
  }
  return `${configuredChannelCount} channels`;
}

export function formatStatsChannelValue(
  channel: StatsChannelDetail,
  channels: GuildChannelOption[],
) {
  return formatGuildChannelValue(
    channel.channelId,
    channels,
    "Channel no longer available",
  );
}

export function formatStatsChannelLabel(channel: StatsChannelDetail) {
  return channel.label === "" ? "No label" : channel.label;
}

export function formatStatsChannelAudience(channel: StatsChannelDetail) {
  if (channel.roleId !== "") {
    return "Specific role";
  }

  switch (normalizeMemberType(channel.memberType)) {
    case "bots":
      return "Bot members";
    case "humans":
      return "Human members";
    default:
      return "All members";
  }
}

export function formatStatsChannelTemplate(channel: StatsChannelDetail) {
  return channel.nameTemplate === "" ? "Default pattern" : channel.nameTemplate;
}

function normalizeStatsIntervalMins(value: number) {
  return value > 0 ? value : defaultStatsIntervalMins;
}

function normalizeMemberType(value: string) {
  const normalized = value.trim().toLowerCase();
  switch (normalized) {
    case "bot":
    case "bots":
      return "bots";
    case "human":
    case "humans":
      return "humans";
    default:
      return "all";
  }
}

function readStatsChannelList(value: unknown) {
  if (!Array.isArray(value)) {
    return [];
  }

  return value
    .map((item) => readStatsChannel(item))
    .filter((item): item is StatsChannelDetail => item !== null);
}

function readStatsChannel(value: unknown): StatsChannelDetail | null {
  if (typeof value !== "object" || value === null) {
    return null;
  }

  const entry = value as Record<string, unknown>;
  return {
    channelId: readString(entry.channel_id),
    label: readString(entry.label),
    nameTemplate: readString(entry.name_template),
    memberType: readString(entry.member_type),
    roleId: readString(entry.role_id),
  };
}

function readBoolean(value: unknown) {
  return value === true;
}

function readNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function readString(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}
