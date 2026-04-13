import type { FeatureAreaID, FeatureRecord } from "../../api/control";
import contractData from "./featureContract.json";

type FeatureTag = string;
type FeatureNavigation = "primary" | "advanced";

interface FeatureAreaContract {
  id: FeatureAreaID;
  anchor: string;
  label: string;
  description: string;
  homeShortcutLabel: string;
  navigation: FeatureNavigation;
}

interface FeatureContractEntry {
  id: string;
  area: FeatureAreaID;
  tags: FeatureTag[];
}

const featureAreas = contractData.areas as FeatureAreaContract[];
const featureEntries = contractData.features as FeatureContractEntry[];

const featureEntryByID = new Map(
  featureEntries.map((entry) => [entry.id, entry] as const),
);

const featureIDsByTag = new Map<FeatureTag, string[]>();
for (const entry of featureEntries) {
  for (const tag of normalizeTags(entry.tags)) {
    const current = featureIDsByTag.get(tag) ?? [];
    current.push(entry.id);
    featureIDsByTag.set(tag, current);
  }
}

export const featureTags = {
  commandsPrimary: "commands.primary",
  commandsAdmin: "commands.admin",
  moderationAutomod: "moderation.automod",
  moderationMuteRole: "moderation.mute_role",
  moderationCommand: "moderation.command",
  moderationRoute: "moderation.route",
  rolesAutoAssignment: "roles.auto_assignment",
  rolesAdvanced: "roles.advanced",
  rolesPresenceWatchBot: "roles.presence_watch.bot",
  rolesPresenceWatchUser: "roles.presence_watch.user",
  rolesPermissionMirror: "roles.permission_mirror",
  statsPrimary: "stats.primary",
  homeCommands: "home.commands",
  homeAdminCommands: "home.admin_commands",
  homeStats: "home.stats",
  homeAutoRole: "home.auto_role",
} as const;

export function getFeatureContractEntry(featureID: string) {
  return featureEntryByID.get(featureID) ?? null;
}

export function getFeatureContractEntries() {
  return featureEntries.map((entry) => ({
    id: entry.id,
    area: entry.area,
    tags: [...entry.tags],
  }));
}

export function getFeatureIDsByTag(tag: string): string[] {
  return [...(featureIDsByTag.get(tag) ?? [])];
}

export function getFeatureAreaID(
  feature: Pick<FeatureRecord, "id" | "area"> | string,
): FeatureAreaID | null {
  if (typeof feature === "string") {
    return getFeatureContractEntry(feature)?.area ?? null;
  }

  const explicitArea = feature.area?.trim();
  if (isFeatureAreaID(explicitArea)) {
    return explicitArea;
  }

  return getFeatureContractEntry(feature.id)?.area ?? null;
}

export function getFeatureTags(
  feature: Pick<FeatureRecord, "id" | "tags">,
): string[] {
  const explicitTags = normalizeTags(feature.tags ?? []);
  if (explicitTags.length > 0) {
    return explicitTags;
  }

  return normalizeTags(getFeatureContractEntry(feature.id)?.tags ?? []);
}

export function featureHasTag(
  feature: Pick<FeatureRecord, "id" | "tags">,
  tag: string,
) {
  return getFeatureTags(feature).includes(tag);
}

export function findFeatureByTag(features: FeatureRecord[], tag: string) {
  return features.find((feature) => featureHasTag(feature, tag)) ?? null;
}

export function filterFeaturesByTag(features: FeatureRecord[], tag: string) {
  return features.filter((feature) => featureHasTag(feature, tag));
}

export function getFeatureAreaContracts() {
  return featureAreas.map((area) => ({ ...area }));
}

function isFeatureAreaID(value: string | undefined): value is FeatureAreaID {
  return featureAreas.some((area) => area.id === value);
}

function normalizeTags(tags: readonly string[]) {
  const normalized = tags
    .map((tag) => tag.trim())
    .filter((tag) => tag !== "");
  return Array.from(new Set(normalized));
}
