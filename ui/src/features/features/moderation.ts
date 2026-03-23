import type { FeatureRecord } from "../../api/control";

interface AutomodFeatureDetails {
  rulesetCount: number;
  looseRuleCount: number;
  blocklistCount: number;
}

export function getAutomodFeatureDetails(
  feature: FeatureRecord,
): AutomodFeatureDetails {
  const details = feature.details ?? {};

  return {
    rulesetCount: readNumber(details.ruleset_count),
    looseRuleCount: readNumber(details.loose_rule_count),
    blocklistCount: readNumber(details.blocklist_count),
  };
}

export function getModerationLogFeatures(features: FeatureRecord[]) {
  return features.filter((feature) => feature.id !== "services.automod");
}

export function summarizeAutomodSignal(feature: FeatureRecord) {
  const details = getAutomodFeatureDetails(feature);
  const firstBlocker = feature.blockers?.[0]?.message?.trim() ?? "";

  if (!feature.effective_enabled) {
    return "Automatic moderation is currently off for the selected server.";
  }

  if (firstBlocker !== "") {
    return firstBlocker;
  }

  if (totalAutomodRuleSources(details) === 0) {
    return "Configure rules before relying on automatic moderation.";
  }

  return "Automatic moderation is configured for the selected server.";
}

export function formatAutomodRuleCoverageValue(feature: FeatureRecord) {
  const details = getAutomodFeatureDetails(feature);
  const total = totalAutomodRuleSources(details);

  if (total === 0) {
    return "No rules configured";
  }
  if (details.rulesetCount > 0) {
    return details.rulesetCount === 1 ? "1 ruleset" : `${details.rulesetCount} rulesets`;
  }
  if (details.looseRuleCount > 0) {
    return details.looseRuleCount === 1
      ? "1 loose rule"
      : `${details.looseRuleCount} loose rules`;
  }
  if (details.blocklistCount === 1) {
    return "1 blocklist";
  }
  return `${details.blocklistCount} blocklists`;
}

export function summarizeAutomodRuleInventory(feature: FeatureRecord) {
  const details = getAutomodFeatureDetails(feature);
  return `${details.rulesetCount} rulesets, ${details.looseRuleCount} loose rules, ${details.blocklistCount} blocklists`;
}

export function formatModerationRouteCoverageValue(features: FeatureRecord[]) {
  const configured = features.filter(
    (feature) => readLoggingChannelID(feature) !== "",
  ).length;
  return `${configured}/${features.length} routes`;
}

function totalAutomodRuleSources(details: AutomodFeatureDetails) {
  return details.rulesetCount + details.looseRuleCount + details.blocklistCount;
}

function readLoggingChannelID(feature: FeatureRecord) {
  const details = feature.details ?? {};
  return typeof details.channel_id === "string" ? details.channel_id.trim() : "";
}

function readNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}
