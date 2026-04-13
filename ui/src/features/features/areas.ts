import type { FeatureAreaID, FeatureRecord } from "../../api/control";
import {
  getFeatureAreaContracts,
  getFeatureAreaID,
  getFeatureContractEntries,
} from "./featureContract";

export interface FeatureAreaDefinition {
  id: FeatureAreaID;
  anchor: string;
  label: string;
  description: string;
  homeShortcutLabel: string;
  navigation: "primary" | "advanced";
  featureIDs: string[];
}

export interface PlannedModuleDefinition {
  id: string;
  label: string;
  description: string;
}

const featureIDsByArea = getFeatureContractEntries().reduce<
  Map<FeatureAreaID, string[]>
>((areas, entry) => {
  const current = areas.get(entry.area) ?? [];
  current.push(entry.id);
  areas.set(entry.area, current);
  return areas;
}, new Map());

export const featureAreaDefinitions: FeatureAreaDefinition[] =
  getFeatureAreaContracts().map((area) => ({
    ...area,
    featureIDs: [...(featureIDsByArea.get(area.id) ?? [])],
  }));

export const primaryFeatureAreaDefinitions = featureAreaDefinitions.filter(
  (area) => area.navigation === "primary",
);

export const advancedFeatureAreaDefinitions = featureAreaDefinitions.filter(
  (area) => area.navigation === "advanced",
);

export const advancedSettingsFeatureIDs = Array.from(
  new Set(advancedFeatureAreaDefinitions.flatMap((area) => area.featureIDs)),
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
  return features.filter((feature) => getFeatureAreaID(feature) === areaID);
}

export function getAdvancedFeatureRecords(
  features: FeatureRecord[],
): FeatureRecord[] {
  return features.filter((feature) => {
    const areaID = getFeatureAreaID(feature);
    return areaID !== null && advancedSettingsFeatureIDs.includes(feature.id);
  });
}
