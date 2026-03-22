import type { FeatureRecord, GuildRoleOption } from "../../api/control";
import { formatRoleOptionLabel, formatRoleValue } from "./roles";

interface CommandsFeatureDetails {
  channelId: string;
}

interface AdminCommandsFeatureDetails {
  allowedRoleIds: string[];
  allowedRoleCount: number;
}

export function getCommandsFeatureDetails(
  feature: FeatureRecord,
): CommandsFeatureDetails {
  const details = feature.details ?? {};

  return {
    channelId: readString(details.channel_id),
  };
}

export function getAdminCommandsFeatureDetails(
  feature: FeatureRecord,
): AdminCommandsFeatureDetails {
  const details = feature.details ?? {};
  const allowedRoleIds = readStringList(details.allowed_role_ids);
  const reportedCount = readNumber(details.allowed_role_count);

  return {
    allowedRoleIds,
    allowedRoleCount:
      reportedCount > 0 ? reportedCount : allowedRoleIds.length,
  };
}

export function canEditCommandsChannel(feature: FeatureRecord) {
  return feature.editable_fields?.includes("channel_id") ?? false;
}

export function canEditAdminCommands(feature: FeatureRecord) {
  return feature.editable_fields?.includes("allowed_role_ids") ?? false;
}

export function formatCommandChannelValue(
  channelId: string,
  emptyLabel = "No dedicated channel",
) {
  const normalizedChannelId = channelId.trim();
  return normalizedChannelId === "" ? emptyLabel : normalizedChannelId;
}

export function formatAllowedRoleCountValue(feature: FeatureRecord) {
  const details = getAdminCommandsFeatureDetails(feature);
  if (details.allowedRoleCount === 0) {
    return "No roles selected";
  }
  if (details.allowedRoleCount === 1) {
    return "1 role";
  }
  return `${details.allowedRoleCount} roles`;
}

export function formatAllowedRolesValue(
  feature: FeatureRecord,
  roleOptions: GuildRoleOption[],
) {
  const details = getAdminCommandsFeatureDetails(feature);
  if (details.allowedRoleCount === 0) {
    return "No roles selected";
  }
  if (roleOptions.length === 0) {
    return `${details.allowedRoleCount} roles configured`;
  }

  return details.allowedRoleIds
    .map((roleId) => formatRoleValue(roleId, roleOptions, "Role no longer available"))
    .join(", ");
}

export function summarizeCommandsSignal(feature: FeatureRecord) {
  const details = getCommandsFeatureDetails(feature);

  if (!feature.effective_enabled) {
    return "Command handling is currently off for the selected server.";
  }

  if ((feature.blockers?.length ?? 0) > 0) {
    return feature.blockers?.[0]?.message ?? "Command handling needs review.";
  }

  if (details.channelId === "") {
    return "Commands are enabled without a dedicated command channel.";
  }

  return "Commands are ready and routed to the configured channel.";
}

export function summarizeAdminCommandsSignal(feature: FeatureRecord) {
  const details = getAdminCommandsFeatureDetails(feature);

  if (!feature.effective_enabled) {
    return "Privileged command workflows are currently off for the selected server.";
  }

  if ((feature.blockers?.length ?? 0) > 0) {
    return feature.blockers?.[0]?.message ?? "Admin command access needs review.";
  }

  if (details.allowedRoleCount === 0) {
    return "Choose the roles that should be allowed to use privileged commands.";
  }

  return "Privileged command access is configured for the selected server.";
}

export function formatCommandServerSetting(feature: FeatureRecord) {
  return feature.override_state === "inherit" ? "Using default" : "Configured here";
}

export function toggleAllowedRole(
  currentRoleIds: string[],
  roleId: string,
) {
  const next = new Set(
    currentRoleIds
      .map((value) => value.trim())
      .filter((value) => value !== ""),
  );

  if (next.has(roleId)) {
    next.delete(roleId);
  } else {
    next.add(roleId);
  }

  return Array.from(next);
}

export function formatAllowedRoleOptionLabel(role: GuildRoleOption) {
  return formatRoleOptionLabel(role);
}

function readString(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function readNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function readStringList(value: unknown) {
  if (!Array.isArray(value)) {
    return [];
  }

  return value
    .filter((item): item is string => typeof item === "string")
    .map((item) => item.trim())
    .filter((item) => item !== "");
}
