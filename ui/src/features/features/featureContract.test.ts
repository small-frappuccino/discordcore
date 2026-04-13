import { describe, expect, it } from "vitest";
import type { FeatureRecord } from "../../api/control";
import {
  featureHasTag,
  featureTags,
  findFeatureByTag,
  getFeatureAreaID,
} from "./featureContract";

function buildFeatureRecord(
  overrides: Partial<FeatureRecord> = {},
): FeatureRecord {
  return {
    id: "services.commands",
    category: "services",
    label: "Commands",
    description: "Command handling",
    scope: "guild",
    supports_guild_override: true,
    override_state: "inherit",
    effective_enabled: true,
    effective_source: "guild",
    readiness: "ready",
    blockers: [],
    details: {},
    editable_fields: ["enabled", "channel_id"],
    ...overrides,
  };
}

describe("feature contract helpers", () => {
  it("falls back to the shared contract area when the payload omits area", () => {
    expect(getFeatureAreaID(buildFeatureRecord())).toBe("commands");
  });

  it("falls back to the shared contract tags when the payload omits tags", () => {
    const feature = buildFeatureRecord({ id: "stats_channels" });

    expect(featureHasTag(feature, featureTags.statsPrimary)).toBe(true);
    expect(featureHasTag(feature, featureTags.homeStats)).toBe(true);
  });

  it("prefers explicit payload tags when they are present", () => {
    const feature = buildFeatureRecord({
      tags: ["custom.tag"],
    });

    expect(featureHasTag(feature, featureTags.commandsPrimary)).toBe(false);
    expect(featureHasTag(feature, "custom.tag")).toBe(true);
  });

  it("finds page-specific features by shared contract tag", () => {
    const features = [
      buildFeatureRecord({ id: "services.commands" }),
      buildFeatureRecord({
        id: "services.admin_commands",
        editable_fields: ["enabled", "allowed_role_ids"],
      }),
    ];

    expect(findFeatureByTag(features, featureTags.commandsAdmin)?.id).toBe(
      "services.admin_commands",
    );
  });
});
