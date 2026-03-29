import type { FeatureAreaID } from "../features/features/areas";

export const appRoutes = {
  landing: "/",
  dashboard: "/dashboard",
  dashboardHome: "/dashboard/home",
  dashboardOverview: "/dashboard/overview",
  dashboardHomePlanned: "/dashboard/home#planned",
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
  settingsPermissions: "/dashboard/settings#permissions",
  settingsAdvanced: "/dashboard/settings#advanced",
} as const;

export interface SidebarNavItem {
  label: string;
  to: string;
  activePath?: string;
  hashes?: string[];
  matchPrefix?: string;
}

export interface SidebarNavSection {
  label: string;
  items: SidebarNavItem[];
}

export const sidebarHomeItem: SidebarNavItem = {
  label: "Home",
  to: appRoutes.dashboardHome,
  activePath: appRoutes.dashboardHome,
};

export const sidebarNavigationSections: SidebarNavSection[] = [
  {
    label: "Core",
    items: [
      {
        label: "Control Panel",
        to: appRoutes.dashboardCoreControlPanel,
        activePath: appRoutes.settings,
        hashes: ["#permissions"],
      },
      {
        label: "Stats",
        to: appRoutes.dashboardCoreStats,
        activePath: appRoutes.dashboardCoreStats,
      },
      {
        label: "Commands",
        to: appRoutes.dashboardCoreCommands,
        activePath: appRoutes.dashboardCoreCommands,
      },
    ],
  },
  {
    label: "Moderation",
    items: [
      {
        label: "Moderation",
        to: appRoutes.dashboardModerationModeration,
        activePath: appRoutes.dashboardModerationModeration,
      },
      {
        label: "Logging",
        to: appRoutes.dashboardModerationLogging,
        activePath: appRoutes.dashboardModerationLogging,
      },
    ],
  },
  {
    label: "Partner Board",
    items: [
      {
        label: "Partner Board",
        to: appRoutes.partnerBoardEntries,
        activePath: appRoutes.partnerBoardEntries,
        matchPrefix: appRoutes.partnerBoardBase,
      },
    ],
  },
  {
    label: "Roles",
    items: [
      {
        label: "Autorole",
        to: appRoutes.dashboardRolesAutorole,
        activePath: appRoutes.dashboardRolesAutorole,
      },
      {
        label: "Level Roles",
        to: appRoutes.dashboardRolesLevelRoles,
        activePath: appRoutes.dashboardRolesLevelRoles,
      },
    ],
  },
];

export const sidebarSettingsItem: SidebarNavItem = {
  label: "Settings",
  to: appRoutes.settings,
  activePath: appRoutes.settings,
};

export const featureAreaRoutes = {
  commands: appRoutes.dashboardCoreCommands,
  moderation: appRoutes.dashboardModerationModeration,
  logging: appRoutes.dashboardModerationLogging,
  roles: appRoutes.dashboardRolesAutorole,
  maintenance: appRoutes.settingsAdvanced,
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
