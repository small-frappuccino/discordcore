import type { FeatureRecord, GuildRoleOption } from "../../api/control";
import { formatRoleOptionLabel, formatRoleValue } from "./roles";

export interface BackfillFeatureDetails {
  channelId: string;
  startDay: string;
  initialDate: string;
}

export interface UserPruneFeatureDetails {
  configEnabled: boolean;
  graceDays: number;
  scanIntervalMins: number;
  initialDelaySecs: number;
  kicksPerSecond: number;
  maxKicksPerRun: number;
  exemptRoleIds: string[];
  exemptRoleCount: number;
  dryRun: boolean;
}

export function getBackfillFeatureDetails(
  feature: FeatureRecord,
): BackfillFeatureDetails {
  const details = feature.details ?? {};

  return {
    channelId: readString(details.channel_id),
    startDay: readString(details.start_day),
    initialDate: readString(details.initial_date),
  };
}

export function getUserPruneFeatureDetails(
  feature: FeatureRecord,
): UserPruneFeatureDetails {
  const details = feature.details ?? {};
  const exemptRoleIds = readStringList(details.exempt_role_ids);
  const reportedCount = readNumber(details.exempt_role_count);

  return {
    configEnabled: readBoolean(details.config_enabled),
    graceDays: readNumber(details.grace_days),
    scanIntervalMins: readNumber(details.scan_interval_mins),
    initialDelaySecs: readNumber(details.initial_delay_secs),
    kicksPerSecond: readNumber(details.kicks_per_second),
    maxKicksPerRun: readNumber(details.max_kicks_per_run),
    exemptRoleIds,
    exemptRoleCount: Math.max(reportedCount, exemptRoleIds.length),
    dryRun: readBoolean(details.dry_run),
  };
}

export function canEditBackfill(feature: FeatureRecord) {
  const fields = feature.editable_fields ?? [];
  return (
    fields.includes("channel_id") ||
    fields.includes("start_day") ||
    fields.includes("initial_date")
  );
}

export function canEditUserPrune(feature: FeatureRecord) {
  const fields = feature.editable_fields ?? [];
  return (
    fields.includes("config_enabled") ||
    fields.includes("grace_days") ||
    fields.includes("scan_interval_mins") ||
    fields.includes("initial_delay_secs") ||
    fields.includes("kicks_per_second") ||
    fields.includes("max_kicks_per_run") ||
    fields.includes("exempt_role_ids") ||
    fields.includes("dry_run")
  );
}

export function summarizeBackfillSignal(feature: FeatureRecord) {
  const details = getBackfillFeatureDetails(feature);
  const blockerMessage = feature.blockers?.[0]?.message ?? "";

  if (!feature.effective_enabled) {
    return "Entry and exit backfill is currently off for the selected server.";
  }

  if (blockerMessage !== "") {
    return blockerMessage;
  }

  if (details.channelId === "") {
    return "Choose the source channel that should seed the backfill run.";
  }

  if (details.startDay === "" && details.initialDate === "") {
    return "Set start day or initial date before relying on backfill.";
  }

  return "Backfill is ready to run with the current channel and schedule seed.";
}

export function summarizeUserPruneSignal(feature: FeatureRecord) {
  const blockerMessage = feature.blockers?.[0]?.message ?? "";
  const details = getUserPruneFeatureDetails(feature);

  if (!feature.effective_enabled) {
    return "User prune is currently off for the selected server.";
  }

  if (blockerMessage !== "") {
    return blockerMessage;
  }

  if (!details.configEnabled) {
    return "Turn on the prune rule before relying on scheduled prune runs.";
  }

  return details.dryRun
    ? "User prune is configured in simulation mode."
    : "User prune is configured for live prune runs.";
}

export function formatBackfillScheduleValue(details: BackfillFeatureDetails) {
  if (details.startDay !== "" && details.initialDate !== "") {
    return `${details.startDay} + ${details.initialDate}`;
  }
  if (details.startDay !== "") {
    return details.startDay;
  }
  if (details.initialDate !== "") {
    return details.initialDate;
  }
  return "Not configured";
}

export function formatUserPruneRuleValue(configEnabled: boolean) {
  return configEnabled ? "Enabled" : "Disabled";
}

export function formatUserPruneRunModeValue(dryRun: boolean) {
  return dryRun ? "Simulation mode" : "Live run";
}

export function formatUserPruneExemptRoleCountValue(
  details: UserPruneFeatureDetails,
) {
  if (details.exemptRoleCount === 0) {
    return "No exempt roles";
  }
  if (details.exemptRoleCount === 1) {
    return "1 exempt role";
  }
  return `${details.exemptRoleCount} exempt roles`;
}

export function formatUserPruneExemptRolesValue(
  details: UserPruneFeatureDetails,
  roles: GuildRoleOption[],
) {
  if (details.exemptRoleCount === 0) {
    return "No exempt roles";
  }
  if (roles.length === 0) {
    return `${details.exemptRoleCount} exempt roles configured`;
  }

  return details.exemptRoleIds
    .map((roleId) => formatRoleValue(roleId, roles, "Role no longer available"))
    .join(", ");
}

export function formatUserPruneRoleOptionLabel(role: GuildRoleOption) {
  return formatRoleOptionLabel(role);
}

export function toggleExemptRole(currentRoleIds: string[], roleId: string) {
  const next = new Set(
    currentRoleIds.map((value) => value.trim()).filter((value) => value !== ""),
  );

  if (next.has(roleId)) {
    next.delete(roleId);
  } else {
    next.add(roleId);
  }

  return Array.from(next);
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

function readStringList(value: unknown) {
  if (!Array.isArray(value)) {
    return [];
  }

  return value
    .filter((item): item is string => typeof item === "string")
    .map((item) => item.trim())
    .filter((item) => item !== "");
}
