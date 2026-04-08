import { describe, expect, it } from "vitest";
import {
  advancedSettingsFeatureIDs,
  featureAreaDefinitions,
  getFeatureAreaDefinition,
} from "./areas";
import { moderationCommandFeatureIDs } from "./moderation";

describe("feature area definitions", () => {
  it("assigns each feature id to only one area", () => {
    const seen = new Set<string>();

    for (const area of featureAreaDefinitions) {
      for (const featureID of area.featureIDs) {
        expect(seen.has(featureID)).toBe(false);
        seen.add(featureID);
      }
    }
  });

  it("keeps operational monitoring in the advanced feature registry", () => {
    expect(getFeatureAreaDefinition("maintenance")?.featureIDs).toContain(
      "services.monitoring",
    );
    expect(advancedSettingsFeatureIDs).toContain("services.monitoring");
  });

  it("exposes moderation command toggles in the moderation area", () => {
    const moderationArea = getFeatureAreaDefinition("moderation");
    expect(moderationArea).not.toBeNull();

    for (const featureID of moderationCommandFeatureIDs) {
      expect(moderationArea?.featureIDs).toContain(featureID);
    }
  });
});
