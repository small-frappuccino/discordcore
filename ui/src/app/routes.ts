export const appRoutes = {
  landing: "/",
  dashboard: "/dashboard",
  dashboardOverview: "/dashboard/overview",
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
  path: string;
  matchPrefix?: string;
}

export const sidebarItems: SidebarItem[] = [
  {
    label: "Overview",
    path: appRoutes.dashboardOverview,
  },
  {
    label: "Partner Board",
    path: appRoutes.partnerBoardEntries,
    matchPrefix: appRoutes.partnerBoardBase,
  },
  {
    label: "Settings",
    path: appRoutes.settings,
  },
];

export const partnerBoardTabs = [
  { label: "Entries", path: appRoutes.partnerBoardEntries },
  { label: "Layout", path: appRoutes.partnerBoardLayout },
  { label: "Destination", path: appRoutes.partnerBoardDelivery },
] as const;
