import { describe, expect, it } from "vitest";
import type { FeatureRecord } from "../../api/control";
import {
  getModerationCommandFeatures,
  getModerationLogFeatures,
  moderationCommandFeatureIDs,
} from "./moderation";

function buildFeatureRecord(
  overrides: Partial<FeatureRecord> = {},
): FeatureRecord {
  return {
    id: "services.automod",
    category: "services",
    label: "Automod service",
    description: "Record AutoMod executions",
    scope: "guild",
    supports_guild_override: true,
    override_state: "enabled",
    effective_enabled: true,
    effective_source: "guild",
    readiness: "ready",
    blockers: [],
    details: {},
    editable_fields: ["enabled"],
    ...overrides,
  };
}

describe("moderation feature helpers", () => {
  it("separates moderation command toggles from route logging features", () => {
    const features = [
      buildFeatureRecord({ id: "services.automod" }),
      buildFeatureRecord({ id: "moderation.ban", category: "moderation" }),
      buildFeatureRecord({ id: "moderation.warn", category: "moderation" }),
      buildFeatureRecord({
        id: "logging.automod_action",
        category: "logging",
        editable_fields: ["enabled", "channel_id"],
      }),
      buildFeatureRecord({
        id: "logging.moderation_case",
        category: "logging",
        editable_fields: ["enabled", "channel_id"],
      }),
    ];

    expect(getModerationCommandFeatures(features).map((feature) => feature.id)).toEqual([
      "moderation.ban",
      "moderation.warn",
    ]);
    expect(getModerationLogFeatures(features).map((feature) => feature.id)).toEqual([
      "logging.automod_action",
      "logging.moderation_case",
    ]);
  });

  it("lists the moderation command feature ids in the declared command registry", () => {
    expect(moderationCommandFeatureIDs).toEqual([
      "moderation.ban",
      "moderation.massban",
      "moderation.kick",
      "moderation.timeout",
      "moderation.warn",
      "moderation.warnings",
    ]);
  });
});
