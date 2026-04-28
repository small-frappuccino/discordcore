import { describe, expect, it } from "vitest";
import {
  appRoutes,
  buildGuildScopedPath,
  getGuildScopedSubpath,
  mapLegacyDashboardPath,
  mapLegacyDashboardPathForGuild,
} from "./routes";

describe("app route contracts", () => {
  it("keeps /manage as the canonical dashboard base path", () => {
    expect(appRoutes.manage).toBe("/manage");
    expect(appRoutes.dashboardCoreControlPanel("guild-1")).toBe(
      "/manage/guild-1/core/control-panel",
    );
    expect(appRoutes.qotdSettings("guild-1")).toBe(
      "/manage/guild-1/qotd/settings",
    );
    expect(appRoutes.dashboardModerationLogging("guild-1")).toBe(
      "/manage/guild-1/moderation/logging",
    );
  });

  it("keeps legacy /dashboard shortcuts explicit and isolated", () => {
    expect(appRoutes.manageLegacy).toBe("/dashboard");
    expect(appRoutes.legacyControlPanel).toBe("/dashboard/control-panel");
    expect(appRoutes.legacyCommands).toBe("/dashboard/commands");
    expect(appRoutes.legacyLogging).toBe("/dashboard/logging");
    expect(appRoutes.legacyStats).toBe("/dashboard/stats");
  });

  it("maps legacy dashboard aliases onto canonical guild-scoped routes", () => {
    expect(mapLegacyDashboardPathForGuild("/dashboard", "guild-1")).toBe(
      appRoutes.dashboardHome("guild-1"),
    );
    expect(mapLegacyDashboardPathForGuild("/dashboard/control-panel/", "guild-1")).toBe(
      appRoutes.dashboardCoreControlPanel("guild-1"),
    );
    expect(mapLegacyDashboardPathForGuild("/dashboard/commands", "guild-1")).toBe(
      appRoutes.dashboardCoreCommands("guild-1"),
    );
    expect(mapLegacyDashboardPathForGuild("/dashboard/logging/", "guild-1")).toBe(
      appRoutes.dashboardModerationLogging("guild-1"),
    );
    expect(mapLegacyDashboardPathForGuild("/dashboard/qotd/questions", "guild-1")).toBe(
      appRoutes.qotdSettings("guild-1"),
    );
    expect(mapLegacyDashboardPathForGuild("/dashboard/roles-members", "guild-1")).toBe(
      appRoutes.dashboardRolesAutorole("guild-1"),
    );
  });

  it("falls back to the canonical base path when no guild is available", () => {
    expect(mapLegacyDashboardPath("/dashboard/control-panel")).toBe(appRoutes.manage);
    expect(mapLegacyDashboardPathForGuild("/dashboard/commands", "   ")).toBe(
      appRoutes.manage,
    );
  });

  it("builds guild-scoped paths from canonical manage routes", () => {
    expect(getGuildScopedSubpath("/manage/guild-1/core/stats")).toBe("core/stats");
    expect(
      buildGuildScopedPath(" guild with spaces ", "/manage/other-guild/moderation/logging"),
    ).toBe("/manage/guild%20with%20spaces/moderation/logging");
  });
});
