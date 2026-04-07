import { describe, expect, it } from "vitest";
import { formatGuildChannelValue } from "./discordEntities";

describe("formatGuildChannelValue", () => {
  it("does not leak raw channel IDs when the lookup cache is stale", () => {
    expect(formatGuildChannelValue("1234567890", [])).toBe(
      "Channel no longer available",
    );
  });
});
