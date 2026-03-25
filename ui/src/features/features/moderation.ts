import type { FeatureRecord } from "../../api/control";

interface AutomodFeatureDetails {
  mode: string;
}

interface MuteRoleFeatureDetails {
  roleId: string;
}

export function getAutomodFeatureDetails(
  feature: FeatureRecord,
): AutomodFeatureDetails {
  const details = feature.details ?? {};

  return {
    mode: readString(details.mode),
  };
}

export function getMuteRoleFeatureDetails(
  feature: FeatureRecord,
): MuteRoleFeatureDetails {
  const details = feature.details ?? {};

  return {
    roleId: readString(details.role_id),
  };
}

export function getModerationLogFeatures(features: FeatureRecord[]) {
  return features.filter(
    (feature) =>
      feature.id !== "services.automod" &&
      feature.id !== "moderation.mute_role",
  );
}

export function canEditMuteRole(feature: FeatureRecord) {
  return feature.editable_fields?.includes("role_id") ?? false;
}

export function summarizeAutomodSignal(feature: FeatureRecord) {
  const firstBlocker = feature.blockers?.[0]?.message?.trim() ?? "";

  if (!feature.effective_enabled) {
    return "The AutoMod listener is currently off for the selected server.";
  }

  if (firstBlocker !== "") {
    return firstBlocker;
  }

  return "Discord native AutoMod executions are available for logging.";
}

export function formatAutomodModeValue(feature: FeatureRecord) {
  const details = getAutomodFeatureDetails(feature);

  switch (details.mode) {
    case "logging_only":
      return "Logging only";
    default:
      return "Listener";
  }
}

export function summarizeAutomodMode(feature: FeatureRecord) {
  if (getAutomodFeatureDetails(feature).mode === "logging_only") {
    return "Discord still owns the rules. This bot only records executions.";
  }
  return summarizeAutomodSignal(feature);
}

export function summarizeMuteRoleSignal(feature: FeatureRecord) {
  const blockerCode = feature.blockers?.[0]?.code ?? "";

  switch (blockerCode) {
    case "missing_role":
      return "Choose the role that should be applied by the mute command.";
    case "invalid_role":
      return "Choose a different mute role. The current one is no longer available.";
    default:
      if (!feature.effective_enabled) {
        return "Role-based muting is currently off for the selected server.";
      }
      return feature.blockers?.[0]?.message ?? "Mute role is ready for role-based mute actions.";
  }
}

export function formatModerationRouteCoverageValue(features: FeatureRecord[]) {
  const configured = features.filter(
    (feature) => readLoggingChannelID(feature) !== "",
  ).length;
  return `${configured}/${features.length} routes`;
}

function readLoggingChannelID(feature: FeatureRecord) {
  const details = feature.details ?? {};
  return typeof details.channel_id === "string" ? details.channel_id.trim() : "";
}

function readString(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}
