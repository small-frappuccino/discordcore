function encodeGuildID(guildId: string) {
  return encodeURIComponent(guildId.trim());
}

export const appRoutes = {
  landing: "/",
  manage: "/manage",
  manageLegacy: "/dashboard",
  dashboardGuildPattern: "/manage/:guildId",
  dashboardHomePattern: "/manage/:guildId/home",
  dashboardCorePattern: "/manage/:guildId/core",
  dashboardCoreControlPanelPattern: "/manage/:guildId/core/control-panel",
  dashboardCoreStatsPattern: "/manage/:guildId/core/stats",
  dashboardCoreCommandsPattern: "/manage/:guildId/core/commands",
  dashboardModerationPattern: "/manage/:guildId/moderation",
  dashboardModerationModerationPattern:
    "/manage/:guildId/moderation/moderation",
  dashboardModerationLoggingPattern: "/manage/:guildId/moderation/logging",
  dashboardPartnerBoardPattern: "/manage/:guildId/partner-board",
  partnerBoardEntriesPattern: "/manage/:guildId/partner-board/entries",
  partnerBoardLayoutPattern: "/manage/:guildId/partner-board/layout",
  partnerBoardDeliveryPattern: "/manage/:guildId/partner-board/delivery",
  dashboardQOTDPattern: "/manage/:guildId/qotd",
  qotdSettingsPattern: "/manage/:guildId/qotd/settings",
  dashboardRolesPattern: "/manage/:guildId/roles",
  dashboardRolesAutorolePattern: "/manage/:guildId/roles/autorole",
  dashboardRolesLevelRolesPattern: "/manage/:guildId/roles/level-roles",
  dashboardHome: (guildId: string) => `/manage/${encodeGuildID(guildId)}/home`,
  dashboardCore: (guildId: string) => `/manage/${encodeGuildID(guildId)}/core`,
  dashboardCoreControlPanel: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/core/control-panel`,
  dashboardCoreStats: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/core/stats`,
  dashboardCoreCommands: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/core/commands`,
  dashboardModeration: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/moderation`,
  dashboardModerationModeration: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/moderation/moderation`,
  dashboardModerationLogging: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/moderation/logging`,
  dashboardPartnerBoard: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/partner-board`,
  partnerBoardBase: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/partner-board`,
  partnerBoardEntries: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/partner-board/entries`,
  partnerBoardLayout: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/partner-board/layout`,
  partnerBoardDelivery: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/partner-board/delivery`,
  dashboardQOTD: (guildId: string) => `/manage/${encodeGuildID(guildId)}/qotd`,
  qotdBase: (guildId: string) => `/manage/${encodeGuildID(guildId)}/qotd`,
  qotdSettings: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/qotd/settings`,
  dashboardRoles: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/roles`,
  dashboardRolesAutorole: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/roles/autorole`,
  dashboardRolesLevelRoles: (guildId: string) =>
    `/manage/${encodeGuildID(guildId)}/roles/level-roles`,
  legacyControlPanel: "/dashboard/control-panel",
  legacyCommands: "/dashboard/commands",
  legacyModeration: "/dashboard/moderation",
  legacyLogging: "/dashboard/logging",
  legacyRoles: "/dashboard/roles",
  legacyStats: "/dashboard/stats",
} as const;

export function buildPartnerBoardTabs(guildId: string) {
  return [
    { label: "Entries", path: appRoutes.partnerBoardEntries(guildId) },
    { label: "Layout", path: appRoutes.partnerBoardLayout(guildId) },
    { label: "Destination", path: appRoutes.partnerBoardDelivery(guildId) },
  ] as const;
}

export function buildGuildScopedPath(guildId: string, pathname: string) {
  const normalizedGuildID = guildId.trim();
  if (normalizedGuildID === "") {
    return appRoutes.manage;
  }

  const subpath = getGuildScopedSubpath(pathname);
  if (subpath === "") {
    return appRoutes.dashboardHome(normalizedGuildID);
  }

  return `/manage/${encodeGuildID(normalizedGuildID)}/${subpath}`;
}

export function getGuildScopedSubpath(pathname: string) {
  const match = pathname.match(/^\/manage\/[^/]+\/?(.*)$/);
  if (match === null) {
    return "";
  }
  return match[1]?.replace(/^\/+/, "") ?? "";
}

export function mapLegacyDashboardPath(pathname: string) {
  return mapLegacyDashboardPathForGuild(pathname, "");
}

export function mapLegacyDashboardPathForGuild(
  pathname: string,
  guildId: string,
) {
  const normalizedGuildID = guildId.trim();
  if (normalizedGuildID === "") {
    return appRoutes.manage;
  }

  if (
    pathname === appRoutes.manageLegacy ||
    pathname === `${appRoutes.manageLegacy}/`
  ) {
    return appRoutes.dashboardHome(normalizedGuildID);
  }

  const match = pathname.match(/^\/dashboard\/?(.*)$/);
  if (match === null) {
    return appRoutes.manage;
  }

  const rest = normalizeLegacyDashboardSubpath(match[1] ?? "");
  switch (rest) {
    case "":
    case "home":
      return appRoutes.dashboardHome(normalizedGuildID);
    case "control-panel":
      return appRoutes.dashboardCoreControlPanel(normalizedGuildID);
    case "commands":
      return appRoutes.dashboardCoreCommands(normalizedGuildID);
    case "moderation":
      return appRoutes.dashboardModerationModeration(normalizedGuildID);
    case "logging":
      return appRoutes.dashboardModerationLogging(normalizedGuildID);
    case "stats":
      return appRoutes.dashboardCoreStats(normalizedGuildID);
    case "partner-board":
      return appRoutes.dashboardPartnerBoard(normalizedGuildID);
    case "partner-board/entries":
      return appRoutes.partnerBoardEntries(normalizedGuildID);
    case "partner-board/layout":
      return appRoutes.partnerBoardLayout(normalizedGuildID);
    case "partner-board/delivery":
      return appRoutes.partnerBoardDelivery(normalizedGuildID);
    case "qotd":
    case "qotd/settings":
    case "qotd/questions":
    case "qotd/collector":
      return appRoutes.qotdSettings(normalizedGuildID);
    case "roles":
    case "roles-members":
      return appRoutes.dashboardRolesAutorole(normalizedGuildID);
    default:
      return appRoutes.dashboardHome(normalizedGuildID);
  }
}

function normalizeLegacyDashboardSubpath(pathname: string) {
  return pathname.replace(/^\/+/, "").replace(/\/+$/, "");
}
