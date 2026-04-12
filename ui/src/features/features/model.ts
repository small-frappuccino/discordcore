import type { FeatureRecord } from "../../api/control";

export interface FeatureCategoryGroup {
  category: string;
  features: FeatureRecord[];
}

export function groupFeaturesByCategory(
  features: FeatureRecord[],
): FeatureCategoryGroup[] {
  const grouped = new Map<string, FeatureRecord[]>();

  for (const feature of features) {
    const items = grouped.get(feature.category) ?? [];
    items.push(feature);
    grouped.set(feature.category, items);
  }

  return Array.from(grouped.entries())
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([category, categoryFeatures]) => ({
      category,
      features: [...categoryFeatures].sort((left, right) =>
        left.label.localeCompare(right.label),
      ),
    }));
}

export function isFeatureReady(feature: FeatureRecord): boolean {
  return feature.readiness === "ready";
}

export function isFeatureBlocked(feature: FeatureRecord): boolean {
  return feature.readiness === "blocked";
}

export function isFeatureConfigurable(feature: FeatureRecord): boolean {
  return getFeatureEditableFields(feature).length > 0;
}

export function getFeatureEditableFields(feature: FeatureRecord): string[] {
  return feature.editable_fields ?? [];
}

export function featureSupportsField(
  feature: FeatureRecord,
  field: string,
): boolean {
  return getFeatureEditableFields(feature).includes(field);
}

export function featureSupportsAnyField(
  feature: FeatureRecord,
  fields: readonly string[],
): boolean {
  const editableFields = getFeatureEditableFields(feature);
  return fields.some((field) => editableFields.includes(field));
}

export function getFeatureStatusTone(feature: FeatureRecord) {
  if (feature.readiness === "ready") {
    return "success";
  }
  if (feature.readiness === "blocked") {
    return "error";
  }
  if (!feature.effective_enabled) {
    return "neutral";
  }
  return "info";
}

export function getFeatureBlockerSummary(feature: FeatureRecord): string {
  if ((feature.blockers?.length ?? 0) === 0) {
    return "";
  }
  return feature.blockers?.map((blocker) => blocker.message).join(" ") ?? "";
}

export function getFeaturePrimaryValue(feature: FeatureRecord): string {
  if (!feature.effective_enabled) {
    return "Disabled";
  }
  if (feature.readiness === "ready") {
    return "Ready";
  }
  if (feature.readiness === "blocked") {
    return "Blocked";
  }
  return feature.readiness;
}
