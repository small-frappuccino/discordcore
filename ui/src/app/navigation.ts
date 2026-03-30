import { appRoutes } from "./routes";

export interface NavigationItem {
  id: string;
  label: string;
  to: string;
  activePath?: string;
  matchPrefix?: string;
  homeActionLabel?: string;
}

export interface NavigationSection {
  id: string;
  label: string;
  items: NavigationItem[];
}

export const dashboardHomeNavigationItem: NavigationItem = {
  id: "home",
  label: "Home",
  to: appRoutes.dashboardHome,
  activePath: appRoutes.dashboardHome,
};

export const dashboardPartnerBoardNavigationItem: NavigationItem = {
  id: "partner-board",
  label: "Partner Board",
  to: appRoutes.partnerBoardBase,
  activePath: appRoutes.partnerBoardBase,
  matchPrefix: appRoutes.partnerBoardBase,
  homeActionLabel: "Open Partner Board",
};

const coreNavigationSection: NavigationSection = {
  id: "core",
  label: "Core",
  items: [
    {
      id: "control-panel",
      label: "Control Panel",
      to: appRoutes.dashboardCoreControlPanel,
      activePath: appRoutes.dashboardCoreControlPanel,
      homeActionLabel: "Open Control Panel",
    },
    {
      id: "stats",
      label: "Stats",
      to: appRoutes.dashboardCoreStats,
      activePath: appRoutes.dashboardCoreStats,
      homeActionLabel: "Open Stats",
    },
    {
      id: "commands",
      label: "Commands",
      to: appRoutes.dashboardCoreCommands,
      activePath: appRoutes.dashboardCoreCommands,
      homeActionLabel: "Open Commands",
    },
  ],
};

const moderationNavigationSection: NavigationSection = {
  id: "moderation",
  label: "Moderation",
  items: [
    {
      id: "moderation",
      label: "Moderation",
      to: appRoutes.dashboardModerationModeration,
      activePath: appRoutes.dashboardModerationModeration,
      homeActionLabel: "Open Moderation",
    },
    {
      id: "logging",
      label: "Logging",
      to: appRoutes.dashboardModerationLogging,
      activePath: appRoutes.dashboardModerationLogging,
      homeActionLabel: "Open Logging",
    },
  ],
};

const partnersNavigationSection: NavigationSection = {
  id: "partners",
  label: "Partners",
  items: [dashboardPartnerBoardNavigationItem],
};

const rolesNavigationSection: NavigationSection = {
  id: "roles",
  label: "Roles",
  items: [
    {
      id: "autorole",
      label: "Autorole",
      to: appRoutes.dashboardRolesAutorole,
      activePath: appRoutes.dashboardRolesAutorole,
      homeActionLabel: "Open Autorole",
    },
    {
      id: "level-roles",
      label: "Level Roles",
      to: appRoutes.dashboardRolesLevelRoles,
      activePath: appRoutes.dashboardRolesLevelRoles,
      homeActionLabel: "Open Level Roles",
    },
  ],
};

export const dashboardSidebarNavigationSections: NavigationSection[] = [
  coreNavigationSection,
  moderationNavigationSection,
  partnersNavigationSection,
  rolesNavigationSection,
];

export const dashboardHomeNavigationSections: NavigationSection[] = [
  coreNavigationSection,
  moderationNavigationSection,
  partnersNavigationSection,
  rolesNavigationSection,
];

export const dashboardNavigationItems = dashboardHomeNavigationSections.flatMap(
  (section) => section.items,
);

export function isNavigationItemActive(pathname: string, item: NavigationItem) {
  const activePath = item.activePath ?? item.to;
  if (item.matchPrefix !== undefined) {
    return pathname.startsWith(item.matchPrefix);
  }
  return pathname === activePath;
}

export function getActiveNavigationSection(pathname: string) {
  return (
    dashboardSidebarNavigationSections.find((section) =>
      section.items.some((item) => isNavigationItemActive(pathname, item)),
    ) ?? null
  );
}
