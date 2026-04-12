import type {
  FeatureRecord,
  GuildMemberOption,
  GuildRoleOption,
} from "../../api/control";

interface AutoRoleFeatureDetails {
  configEnabled: boolean;
  targetRoleId: string;
  requiredRoleIds: string[];
  requiredRoleCount: number;
  levelRoleId: string;
  boosterRoleId: string;
}

interface PresenceWatchBotDetails {
  watchBot: boolean;
}

interface PresenceWatchUserDetails {
  userId: string;
}

interface PermissionMirrorDetails {
  actorRoleId: string;
  runtimeDisabled: boolean;
}

export function getAutoRoleFeatureDetails(
  feature: FeatureRecord,
): AutoRoleFeatureDetails {
  const details = feature.details ?? {};

  return {
    configEnabled: readBoolean(details.config_enabled),
    targetRoleId: readString(details.target_role_id),
    requiredRoleIds: readStringList(details.required_role_ids),
    requiredRoleCount: readNumber(details.required_role_count),
    levelRoleId: readString(details.level_role_id),
    boosterRoleId: readString(details.booster_role_id),
  };
}

export function getPresenceWatchBotDetails(
  feature: FeatureRecord,
): PresenceWatchBotDetails {
  const details = feature.details ?? {};

  return {
    watchBot: readBoolean(details.watch_bot),
  };
}

export function getPresenceWatchUserDetails(
  feature: FeatureRecord,
): PresenceWatchUserDetails {
  const details = feature.details ?? {};

  return {
    userId: readString(details.user_id),
  };
}

export function getPermissionMirrorDetails(
  feature: FeatureRecord,
): PermissionMirrorDetails {
  const details = feature.details ?? {};

  return {
    actorRoleId: readString(details.actor_role_id),
    runtimeDisabled: readBoolean(details.runtime_disabled),
  };
}

export function formatRoleValue(
  roleId: string,
  roleOptions: GuildRoleOption[],
  emptyLabel = "Not configured",
) {
  const normalizedRoleId = roleId.trim();
  if (normalizedRoleId === "") {
    return emptyLabel;
  }

  const option = roleOptions.find((item) => item.id === normalizedRoleId);
  if (option === undefined) {
    return "Role no longer available";
  }

  if (option.is_default) {
    return "@everyone";
  }

  return option.name;
}

export function formatRequirementRolesValue(
  feature: FeatureRecord,
  roleOptions: GuildRoleOption[],
) {
  const details = getAutoRoleFeatureDetails(feature);
  const names = [
    formatRoleValue(details.levelRoleId, roleOptions, ""),
    formatRoleValue(details.boosterRoleId, roleOptions, ""),
  ].filter((value) => value !== "");

  if (names.length === 0) {
    return "Not configured";
  }

  return names.join(" + ");
}

export function summarizeAutoRoleSignal(feature: FeatureRecord) {
  const blockerCode = feature.blockers?.[0]?.code ?? "";

  switch (blockerCode) {
    case "config_disabled":
      return "Turn on the assignment rule before relying on automatic role assignment.";
    case "missing_target_role":
      return "Choose the role that should be assigned automatically.";
    case "invalid_target_role":
      return "Choose a different target role. The current one is no longer available in this server.";
    case "invalid_required_roles":
      return "Choose both requirement roles again. The current requirement set is incomplete or outdated.";
    default:
      if (!feature.effective_enabled) {
        return "The module is currently off for this server.";
      }
      return (
        feature.blockers?.[0]?.message ??
        "Automatic role assignment is ready to review."
      );
  }
}

export function summarizeAdvancedRoleSignal(feature: FeatureRecord) {
  const blockerCode = feature.blockers?.[0]?.code ?? "";

  switch (blockerCode) {
    case "runtime_disabled":
      return "The runtime watch flag is turned off.";
    case "missing_user_id":
      return "Choose the member that should be monitored.";
    case "runtime_kill_switch":
      return "Permission mirroring is disabled at runtime.";
    case "invalid_actor_role":
      return "Choose a different actor role. The current one is no longer available.";
    default:
      if (!feature.effective_enabled) {
        return "This advanced control is currently off for the selected server.";
      }
      return (
        feature.blockers?.[0]?.message ?? "No blockers are currently reported."
      );
  }
}

export function formatRoleOptionLabel(role: GuildRoleOption) {
  const name = role.is_default ? "@everyone" : role.name;
  if (role.managed) {
    return `${name} (managed)`;
  }
  return name;
}

export function formatMemberOptionLabel(member: GuildMemberOption) {
  const displayName = member.display_name.trim();
  const username = member.username.trim();
  const suffix =
    username !== "" && username !== displayName ? ` (@${username})` : "";
  const botSuffix = member.bot ? " • Bot" : "";
  return `${displayName}${suffix}${botSuffix}`;
}

export function formatMemberValue(
  memberId: string,
  memberOptions: GuildMemberOption[],
  emptyLabel = "Not configured",
) {
  const normalizedMemberId = memberId.trim();
  if (normalizedMemberId === "") {
    return emptyLabel;
  }

  const option = memberOptions.find((item) => item.id === normalizedMemberId);
  if (option === undefined) {
    return "Member no longer available";
  }

  return formatMemberOptionLabel(option);
}

export function countEnabledFeatures(features: FeatureRecord[]) {
  return features.filter((feature) => feature.effective_enabled).length;
}

function readBoolean(value: unknown) {
  return typeof value === "boolean" ? value : false;
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
