import type { FeatureRecord } from "../../api/control";

export type FeatureAreaID =
  | "commands"
  | "moderation"
  | "logging"
  | "roles"
  | "maintenance"
  | "stats";

export interface FeatureAreaDefinition {
  id: FeatureAreaID;
  anchor: string;
  label: string;
  description: string;
  navigation: "primary" | "advanced";
  featureIDs: string[];
}

export interface PlannedModuleDefinition {
  id: string;
  label: string;
  description: string;
}

export const featureAreaDefinitions: FeatureAreaDefinition[] = [
  {
    id: "commands",
    anchor: "commands",
    label: "Commands",
    description:
      "Command handling, command routing, and privileged command access for the selected server.",
    navigation: "primary",
    featureIDs: [
      "services.commands",
      "services.admin_commands",
    ],
  },
  {
    id: "moderation",
    anchor: "moderation",
    label: "Moderation",
    description:
      "Logging-only AutoMod readiness, mute-role setup, and moderation event routes for staff workflows.",
    navigation: "primary",
    featureIDs: [
      "services.automod",
      "moderation.mute_role",
      "logging.automod_action",
      "logging.moderation_case",
    ],
  },
  {
    id: "logging",
    anchor: "logging",
    label: "Logging",
    description:
      "User, message, and reaction event logging for day-to-day observability.",
    navigation: "primary",
    featureIDs: [
      "logging.avatar_logging",
      "logging.role_update",
      "logging.member_join",
      "logging.member_leave",
      "logging.message_process",
      "logging.message_edit",
      "logging.message_delete",
      "logging.reaction_metric",
    ],
  },
  {
    id: "roles",
    anchor: "roles",
    label: "Roles",
    description:
      "Role assignment and member-facing safeguards that still depend on role configuration.",
    navigation: "primary",
    featureIDs: [
      "presence_watch.bot",
      "presence_watch.user",
      "safety.bot_role_perm_mirror",
      "auto_role_assignment",
    ],
  },
  {
    id: "maintenance",
    anchor: "maintenance",
    label: "Maintenance",
    description:
      "Advanced cleanup, backfill, pruning, and scheduled data maintenance.",
    navigation: "advanced",
    featureIDs: [
      "message_cache.cleanup_on_startup",
      "message_cache.delete_on_log",
      "maintenance.db_cleanup",
      "backfill.enabled",
      "user_prune",
    ],
  },
  {
    id: "stats",
    anchor: "stats",
    label: "Stats",
    description:
      "Server statistics and member-count channel updates for the selected guild.",
    navigation: "primary",
    featureIDs: ["stats_channels"],
  },
];

export const primaryFeatureAreaDefinitions = featureAreaDefinitions.filter(
  (area) => area.navigation === "primary",
);

export const advancedFeatureAreaDefinitions = featureAreaDefinitions.filter(
  (area) => area.navigation === "advanced",
);

export const plannedModules: PlannedModuleDefinition[] = [
  {
    id: "tickets",
    label: "Tickets",
    description:
      "Planned for a later phase after the support workflow and operator experience are researched properly.",
  },
];

export function getFeatureAreaDefinition(
  areaID: FeatureAreaID,
): FeatureAreaDefinition | null {
  return featureAreaDefinitions.find((area) => area.id === areaID) ?? null;
}

export function getFeatureAreaRecords(
  features: FeatureRecord[],
  areaID: FeatureAreaID,
): FeatureRecord[] {
  const definition = getFeatureAreaDefinition(areaID);
  if (definition === null) {
    return [];
  }

  const featureIDSet = new Set(definition.featureIDs);
  return features.filter((feature) => featureIDSet.has(feature.id));
}
