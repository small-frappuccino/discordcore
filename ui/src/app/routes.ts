import type { FeatureAreaID } from "../features/features/areas";

export const appRoutes = {
  landing: "/",
  dashboard: "/dashboard",
  dashboardHome: "/dashboard/home",
  dashboardOverview: "/dashboard/overview",
  dashboardCore: "/dashboard/core",
  dashboardCoreControlPanel: "/dashboard/core/control-panel",
  dashboardCoreStats: "/dashboard/core/stats",
  dashboardCoreCommands: "/dashboard/core/commands",
  dashboardModeration: "/dashboard/moderation",
  dashboardModerationModeration: "/dashboard/moderation/moderation",
  dashboardModerationLogging: "/dashboard/moderation/logging",
  dashboardPartnerBoard: "/dashboard/partner-board",
  partnerBoardBase: "/dashboard/partner-board",
  partnerBoardEntries: "/dashboard/partner-board/entries",
  partnerBoardLayout: "/dashboard/partner-board/layout",
  partnerBoardDelivery: "/dashboard/partner-board/delivery",
  partnerBoardActivity: "/dashboard/partner-board/activity",
  dashboardRoles: "/dashboard/roles",
  dashboardRolesAutorole: "/dashboard/roles/autorole",
  dashboardRolesLevelRoles: "/dashboard/roles/level-roles",
  controlPanel: "/dashboard/control-panel",
  commands: "/dashboard/commands",
  moderation: "/dashboard/moderation",
  logging: "/dashboard/logging",
  roles: "/dashboard/roles",
  maintenance: "/dashboard/maintenance",
  stats: "/dashboard/stats",
  automations: "/dashboard/automations",
  activity: "/dashboard/activity",
  settings: "/dashboard/settings",
} as const;

export const featureAreaRoutes = {
  commands: appRoutes.dashboardCoreCommands,
  moderation: appRoutes.dashboardModerationModeration,
  logging: appRoutes.dashboardModerationLogging,
  roles: appRoutes.dashboardRolesAutorole,
  maintenance: appRoutes.dashboardHome,
  stats: appRoutes.dashboardCoreStats,
} as const satisfies Record<FeatureAreaID, string>;

export const partnerBoardTabs = [
  { label: "Entries", path: appRoutes.partnerBoardEntries },
  { label: "Layout", path: appRoutes.partnerBoardLayout },
  { label: "Destination", path: appRoutes.partnerBoardDelivery },
] as const;

export function getFeatureAreaRoute(areaId: FeatureAreaID): string {
  return featureAreaRoutes[areaId];
}
