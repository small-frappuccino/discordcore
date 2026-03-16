import type { FeatureRecord } from "../../api/control";

export type FeatureAreaID =
  | "commands"
  | "moderation"
  | "logging"
  | "roles-members"
  | "maintenance"
  | "stats";

export interface FeatureAreaDefinition {
  id: FeatureAreaID;
  anchor: string;
  label: string;
  description: string;
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
      "Command handling, privileged access, and the shared core service that supports command workflows.",
    featureIDs: [
      "services.monitoring",
      "services.commands",
      "services.admin_commands",
    ],
  },
  {
    id: "moderation",
    anchor: "moderation",
    label: "Moderation",
    description:
      "AutoMod controls plus moderation event logging needed for enforcement workflows.",
    featureIDs: [
      "services.automod",
      "logging.automod_action",
      "logging.moderation_case",
      "logging.clean_action",
    ],
  },
  {
    id: "logging",
    anchor: "logging",
    label: "Logging",
    description:
      "User, message, and reaction event logging for day-to-day observability.",
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
    id: "roles-members",
    anchor: "roles-members",
    label: "Roles & Members",
    description:
      "Role automation, presence watching, and member-facing safeguards.",
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
      "Cache cleanup, backfill, pruning, and scheduled data maintenance.",
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
    featureIDs: ["stats_channels"],
  },
];

export const plannedModules: PlannedModuleDefinition[] = [
  {
    id: "tickets",
    label: "Tickets",
    description:
      "Planned for a later phase after the support workflow and operator experience are researched properly.",
  },
];

export function getFeatureAreaRecords(
  features: FeatureRecord[],
  areaID: FeatureAreaID,
): FeatureRecord[] {
  const definition = featureAreaDefinitions.find((area) => area.id === areaID);
  if (definition === undefined) {
    return [];
  }

  const featureIDSet = new Set(definition.featureIDs);
  return features.filter((feature) => featureIDSet.has(feature.id));
}
