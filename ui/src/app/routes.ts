export const appRoutes = {
  landing: "/",
  dashboard: "/dashboard",
  dashboardHome: "/dashboard/home",
  dashboardOverview: "/dashboard/overview",
  dashboardHomeCommands: "/dashboard/home#commands",
  dashboardHomeModeration: "/dashboard/home#moderation",
  dashboardHomeLogging: "/dashboard/home#logging",
  dashboardHomeRolesMembers: "/dashboard/home#roles-members",
  dashboardHomeMaintenance: "/dashboard/home#maintenance",
  dashboardHomeStats: "/dashboard/home#stats",
  dashboardHomePlanned: "/dashboard/home#planned",
  legacyControlPanel: "/dashboard/control-panel",
  partnerBoardBase: "/dashboard/partner-board",
  partnerBoardEntries: "/dashboard/partner-board/entries",
  partnerBoardLayout: "/dashboard/partner-board/layout",
  partnerBoardDelivery: "/dashboard/partner-board/delivery",
  partnerBoardActivity: "/dashboard/partner-board/activity",
  moderation: "/dashboard/moderation",
  automations: "/dashboard/automations",
  activity: "/dashboard/activity",
  settings: "/dashboard/settings",
} as const;

export interface SidebarItem {
  label: string;
  to: string;
  path: string;
  hashes?: string[];
  matchPrefix?: string;
}

export const sidebarItems: SidebarItem[] = [
  {
    label: "Home",
    to: appRoutes.dashboardHome,
    path: appRoutes.dashboardHome,
    hashes: ["", appRoutes.dashboardHomePlanned.replace(appRoutes.dashboardHome, "")],
  },
  {
    label: "Partner Board",
    to: appRoutes.partnerBoardEntries,
    path: appRoutes.partnerBoardEntries,
    matchPrefix: appRoutes.partnerBoardBase,
  },
  {
    label: "Commands",
    to: appRoutes.dashboardHomeCommands,
    path: appRoutes.dashboardHome,
    hashes: ["#commands"],
  },
  {
    label: "Moderation",
    to: appRoutes.dashboardHomeModeration,
    path: appRoutes.dashboardHome,
    hashes: ["#moderation"],
  },
  {
    label: "Logging",
    to: appRoutes.dashboardHomeLogging,
    path: appRoutes.dashboardHome,
    hashes: ["#logging"],
  },
  {
    label: "Roles & Members",
    to: appRoutes.dashboardHomeRolesMembers,
    path: appRoutes.dashboardHome,
    hashes: ["#roles-members"],
  },
  {
    label: "Maintenance",
    to: appRoutes.dashboardHomeMaintenance,
    path: appRoutes.dashboardHome,
    hashes: ["#maintenance"],
  },
  {
    label: "Stats",
    to: appRoutes.dashboardHomeStats,
    path: appRoutes.dashboardHome,
    hashes: ["#stats"],
  },
  {
    label: "Settings",
    to: appRoutes.settings,
    path: appRoutes.settings,
  },
];

export const partnerBoardTabs = [
  { label: "Entries", path: appRoutes.partnerBoardEntries },
  { label: "Layout", path: appRoutes.partnerBoardLayout },
  { label: "Destination", path: appRoutes.partnerBoardDelivery },
] as const;
