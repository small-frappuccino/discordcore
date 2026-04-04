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

export function getDashboardHomeNavigationItem(
  guildId: string,
): NavigationItem {
  return {
    id: "home",
    label: "Home",
    to: appRoutes.dashboardHome(guildId),
    activePath: appRoutes.dashboardHome(guildId),
  };
}

export function getDashboardPartnerBoardNavigationItem(
  guildId: string,
): NavigationItem {
  const partnerBoardBase = appRoutes.partnerBoardBase(guildId);

  return {
    id: "partner-board",
    label: "Partner Board",
    to: partnerBoardBase,
    activePath: partnerBoardBase,
    matchPrefix: partnerBoardBase,
    homeActionLabel: "Open Partner Board",
  };
}

export function getDashboardQOTDNavigationItem(
  guildId: string,
): NavigationItem {
  const qotdBase = appRoutes.qotdBase(guildId);

  return {
    id: "qotd",
    label: "QOTD",
    to: qotdBase,
    activePath: qotdBase,
    matchPrefix: qotdBase,
    homeActionLabel: "Open QOTD",
  };
}

export function getDashboardSidebarNavigationSections(
  guildId: string,
): NavigationSection[] {
  return [
    getDashboardCoreNavigationSection(guildId),
    getDashboardModerationNavigationSection(guildId),
    getDashboardPartnersNavigationSection(guildId),
    getDashboardEngagementNavigationSection(guildId),
    getDashboardRolesNavigationSection(guildId),
  ];
}

export function getDashboardHomeNavigationSections(
  guildId: string,
): NavigationSection[] {
  return [
    getDashboardCoreNavigationSection(guildId),
    getDashboardModerationNavigationSection(guildId),
    getDashboardPartnersNavigationSection(guildId),
    getDashboardEngagementNavigationSection(guildId),
    getDashboardRolesNavigationSection(guildId),
  ];
}

export function getDashboardNavigationItems(guildId: string) {
  return getDashboardHomeNavigationSections(guildId).flatMap(
    (section) => section.items,
  );
}

export function isNavigationItemActive(pathname: string, item: NavigationItem) {
  const activePath = item.activePath ?? item.to;
  if (item.matchPrefix !== undefined) {
    return pathname.startsWith(item.matchPrefix);
  }
  return pathname === activePath;
}

export function getActiveNavigationSection(pathname: string, guildId: string) {
  return (
    getDashboardSidebarNavigationSections(guildId).find((section) =>
      section.items.some((item) => isNavigationItemActive(pathname, item)),
    ) ?? null
  );
}

function getDashboardCoreNavigationSection(guildId: string): NavigationSection {
  return {
    id: "core",
    label: "Core",
    items: [
      {
        id: "control-panel",
        label: "Control Panel",
        to: appRoutes.dashboardCoreControlPanel(guildId),
        activePath: appRoutes.dashboardCoreControlPanel(guildId),
        homeActionLabel: "Open Control Panel",
      },
      {
        id: "stats",
        label: "Stats",
        to: appRoutes.dashboardCoreStats(guildId),
        activePath: appRoutes.dashboardCoreStats(guildId),
        homeActionLabel: "Open Stats",
      },
      {
        id: "commands",
        label: "Commands",
        to: appRoutes.dashboardCoreCommands(guildId),
        activePath: appRoutes.dashboardCoreCommands(guildId),
        homeActionLabel: "Open Commands",
      },
    ],
  };
}

function getDashboardModerationNavigationSection(
  guildId: string,
): NavigationSection {
  return {
    id: "moderation",
    label: "Moderation",
    items: [
      {
        id: "moderation",
        label: "Moderation",
        to: appRoutes.dashboardModerationModeration(guildId),
        activePath: appRoutes.dashboardModerationModeration(guildId),
        homeActionLabel: "Open Moderation",
      },
      {
        id: "logging",
        label: "Logging",
        to: appRoutes.dashboardModerationLogging(guildId),
        activePath: appRoutes.dashboardModerationLogging(guildId),
        homeActionLabel: "Open Logging",
      },
    ],
  };
}

function getDashboardPartnersNavigationSection(
  guildId: string,
): NavigationSection {
  return {
    id: "partners",
    label: "Partners",
    items: [getDashboardPartnerBoardNavigationItem(guildId)],
  };
}

function getDashboardEngagementNavigationSection(
  guildId: string,
): NavigationSection {
  return {
    id: "engagement",
    label: "Engagement",
    items: [getDashboardQOTDNavigationItem(guildId)],
  };
}

function getDashboardRolesNavigationSection(guildId: string): NavigationSection {
  return {
    id: "roles",
    label: "Roles",
    items: [
      {
        id: "autorole",
        label: "Autorole",
        to: appRoutes.dashboardRolesAutorole(guildId),
        activePath: appRoutes.dashboardRolesAutorole(guildId),
        homeActionLabel: "Open Autorole",
      },
      {
        id: "level-roles",
        label: "Level Roles",
        to: appRoutes.dashboardRolesLevelRoles(guildId),
        activePath: appRoutes.dashboardRolesLevelRoles(guildId),
        homeActionLabel: "Open Level Roles",
      },
    ],
  };
}
