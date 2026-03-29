import { appRoutes } from "./routes";

export interface NavigationItem {
  id: string;
  label: string;
  to: string;
  activePath?: string;
  matchPrefix?: string;
  homeTitle?: string;
  homeDescription?: string;
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

export const dashboardNavigationSections: NavigationSection[] = [
  {
    id: "core",
    label: "Core",
    items: [
      {
        id: "control-panel",
        label: "Control Panel",
        to: appRoutes.dashboardCoreControlPanel,
        activePath: appRoutes.dashboardCoreControlPanel,
        homeTitle: "Control Panel",
        homeDescription: "Dashboard access roles and core panel permissions.",
        homeActionLabel: "Open Control Panel",
      },
      {
        id: "stats",
        label: "Stats",
        to: appRoutes.dashboardCoreStats,
        activePath: appRoutes.dashboardCoreStats,
        homeTitle: "Stats",
        homeDescription: "Stats channels, update interval, and posting coverage.",
        homeActionLabel: "Open Stats",
      },
      {
        id: "commands",
        label: "Commands",
        to: appRoutes.dashboardCoreCommands,
        activePath: appRoutes.dashboardCoreCommands,
        homeTitle: "Commands",
        homeDescription: "Command routing and privileged command access.",
        homeActionLabel: "Open Commands",
      },
    ],
  },
  {
    id: "moderation",
    label: "Moderation",
    items: [
      {
        id: "moderation",
        label: "Moderation",
        to: appRoutes.dashboardModerationModeration,
        activePath: appRoutes.dashboardModerationModeration,
        homeTitle: "Moderation",
        homeDescription: "Kick, ban, mute, timeout, and enforcement controls.",
        homeActionLabel: "Open Moderation",
      },
      {
        id: "logging",
        label: "Logging",
        to: appRoutes.dashboardModerationLogging,
        activePath: appRoutes.dashboardModerationLogging,
        homeTitle: "Logging",
        homeDescription: "Moderation and event log destinations for this server.",
        homeActionLabel: "Open Logging",
      },
    ],
  },
  {
    id: "partner-board",
    label: "Partner Board",
    items: [
      {
        id: "partner-board",
        label: "Partner Board",
        to: appRoutes.partnerBoardBase,
        activePath: appRoutes.partnerBoardBase,
        matchPrefix: appRoutes.partnerBoardBase,
        homeTitle: "Partner Board",
        homeDescription: "Entries, layout, and destination for board publishing.",
        homeActionLabel: "Open Partner Board",
      },
    ],
  },
  {
    id: "roles",
    label: "Roles",
    items: [
      {
        id: "autorole",
        label: "Autorole",
        to: appRoutes.dashboardRolesAutorole,
        activePath: appRoutes.dashboardRolesAutorole,
        homeTitle: "Autorole",
        homeDescription: "Automatic role assignment based on selected requirements.",
        homeActionLabel: "Open Autorole",
      },
      {
        id: "level-roles",
        label: "Level Roles",
        to: appRoutes.dashboardRolesLevelRoles,
        activePath: appRoutes.dashboardRolesLevelRoles,
        homeTitle: "Level Roles",
        homeDescription: "Reserved for the upcoming level-role table workflow.",
        homeActionLabel: "Open Level Roles",
      },
    ],
  },
];

export const dashboardNavigationItems = dashboardNavigationSections.flatMap(
  (section) => section.items,
);

export function isNavigationItemActive(pathname: string, item: NavigationItem) {
  const activePath = item.activePath ?? item.to;
  if (item.matchPrefix !== undefined) {
    return pathname.startsWith(item.matchPrefix);
  }
  return pathname === activePath;
}
