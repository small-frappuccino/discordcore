import type { FeatureAreaID } from "../features/features/areas";

export const featureAreaRoutes = {
  commands: "/dashboard/commands",
  moderation: "/dashboard/moderation",
  logging: "/dashboard/logging",
  roles: "/dashboard/roles",
  maintenance: "/dashboard/maintenance",
  stats: "/dashboard/stats",
} as const satisfies Record<FeatureAreaID, string>;

export const appRoutes = {
  landing: "/",
  dashboard: "/dashboard",
  dashboardHome: "/dashboard/home",
  dashboardOverview: "/dashboard/overview",
  dashboardHomePlanned: "/dashboard/home#planned",
  legacyControlPanel: "/dashboard/control-panel",
  partnerBoardBase: "/dashboard/partner-board",
  partnerBoardEntries: "/dashboard/partner-board/entries",
  partnerBoardLayout: "/dashboard/partner-board/layout",
  partnerBoardDelivery: "/dashboard/partner-board/delivery",
  partnerBoardActivity: "/dashboard/partner-board/activity",
  commands: featureAreaRoutes.commands,
  moderation: featureAreaRoutes.moderation,
  logging: featureAreaRoutes.logging,
  roles: featureAreaRoutes.roles,
  maintenance: featureAreaRoutes.maintenance,
  stats: featureAreaRoutes.stats,
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
  },
  {
    label: "Partner Board",
    to: appRoutes.partnerBoardEntries,
    path: appRoutes.partnerBoardEntries,
    matchPrefix: appRoutes.partnerBoardBase,
  },
  {
    label: "Commands",
    to: appRoutes.commands,
    path: appRoutes.commands,
  },
  {
    label: "Moderation",
    to: appRoutes.moderation,
    path: appRoutes.moderation,
  },
  {
    label: "Logging",
    to: appRoutes.logging,
    path: appRoutes.logging,
  },
  {
    label: "Roles",
    to: appRoutes.roles,
    path: appRoutes.roles,
  },
  {
    label: "Stats",
    to: appRoutes.stats,
    path: appRoutes.stats,
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

export function getFeatureAreaRoute(areaId: FeatureAreaID): string {
  return featureAreaRoutes[areaId];
}
