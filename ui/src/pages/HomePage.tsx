import { Link } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import { dashboardNavigationSections, type NavigationItem } from "../app/navigation";
import { AlertBanner, PageHeader, StatusBadge, SurfaceCard } from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { useGuildRolesSettings } from "../features/control-panel/useGuildRolesSettings";
import {
  getAdminCommandsFeatureDetails,
  getCommandsFeatureDetails,
} from "../features/features/commands";
import { getFeatureAreaRecords } from "../features/features/areas";
import { summarizeFeatureArea } from "../features/features/presentation";
import { getAutoRoleFeatureDetails, summarizeAutoRoleSignal } from "../features/features/roles";
import { getStatsFeatureDetails, summarizeStatsSignal } from "../features/features/stats";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { usePartnerBoardSummary } from "../features/partner-board/usePartnerBoardSummary";

interface HomeCardData {
  item: NavigationItem;
  facts: string[];
  loading: boolean;
}

export function HomePage() {
  const { authState, selectedGuild, selectedGuildAccessLevel } = useDashboardSession();
  const workspace = useFeatureWorkspace({
    scope: "guild",
  });
  const rolesSettings = useGuildRolesSettings();
  const partnerBoard = usePartnerBoardSummary();

  const features = workspace.features;
  const statsFeature = features.find((feature) => feature.id === "stats_channels") ?? null;
  const commandsFeature =
    features.find((feature) => feature.id === "services.commands") ?? null;
  const adminCommandsFeature =
    features.find((feature) => feature.id === "services.admin_commands") ?? null;
  const moderationFeatures = getFeatureAreaRecords(features, "moderation");
  const loggingFeatures = getFeatureAreaRecords(features, "logging");
  const autoRoleFeature =
    features.find((feature) => feature.id === "auto_role_assignment") ?? null;

  const homeNotice = rolesSettings.notice ?? workspace.notice ?? partnerBoard.notice;
  const selectedServerLabel = selectedGuild?.name ?? "No server selected";

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Home"
        title="Home"
        description="Browse the dashboard by area. Each card summarizes one page and links directly to its workspace."
        status={
          <StatusBadge tone={selectedGuildAccessLevel === "read" ? "info" : "neutral"}>
            {selectedGuildAccessLevel === "read" ? "Read-only access" : "Navigation index"}
          </StatusBadge>
        }
        meta={<span className="meta-pill subtle-pill">{selectedServerLabel}</span>}
      />

      <AlertBanner notice={homeNotice} />

      <div className="home-category-stack">
        {dashboardNavigationSections.map((section) => (
          <section className="home-category-section" key={section.id}>
            <div className="home-category-header">
              <h2>{section.label}</h2>
            </div>

            <div className="home-card-grid">
              {section.items.map((item) => {
                const card = buildHomeCardData(item, {
                  authState,
                  selectedGuildPresent: selectedGuild !== null,
                  selectedGuildAccessLevel,
                  rolesSettings,
                  partnerBoard,
                  statsFeature,
                  commandsFeature,
                  adminCommandsFeature,
                  moderationFeatures,
                  loggingFeatures,
                  autoRoleFeature,
                  workspaceState: workspace.workspaceState,
                });

                return (
                  <SurfaceCard className="home-nav-card" key={item.id}>
                    {card.loading ? (
                      <div className="home-nav-card-skeleton" aria-hidden="true">
                        <span className="home-nav-skeleton home-nav-skeleton-title" />
                        <span className="home-nav-skeleton" />
                        <span className="home-nav-skeleton" />
                        <span className="home-nav-skeleton home-nav-skeleton-button" />
                      </div>
                    ) : (
                      <>
                        <div className="card-copy">
                          <p className="section-label">{section.label}</p>
                          <h3>{card.item.homeTitle ?? card.item.label}</h3>
                          <p className="section-description">
                            {card.item.homeDescription ?? "Open this area."}
                          </p>
                        </div>

                        <ul className="home-nav-facts">
                          {card.facts.map((fact) => (
                            <li key={fact}>{fact}</li>
                          ))}
                        </ul>

                        <div className="home-nav-card-footer">
                          <Link className="button-primary" to={card.item.to}>
                            {card.item.homeActionLabel ?? `Open ${card.item.label}`}
                          </Link>
                        </div>
                      </>
                    )}
                  </SurfaceCard>
                );
              })}
            </div>
          </section>
        ))}
      </div>
    </section>
  );
}

function buildHomeCardData(
  item: NavigationItem,
  context: {
    authState: string;
    selectedGuildPresent: boolean;
    selectedGuildAccessLevel: string | null;
    rolesSettings: ReturnType<typeof useGuildRolesSettings>;
    partnerBoard: ReturnType<typeof usePartnerBoardSummary>;
    statsFeature: FeatureRecord | null;
    commandsFeature: FeatureRecord | null;
    adminCommandsFeature: FeatureRecord | null;
    moderationFeatures: FeatureRecord[];
    loggingFeatures: FeatureRecord[];
    autoRoleFeature: FeatureRecord | null;
    workspaceState: ReturnType<typeof useFeatureWorkspace>["workspaceState"];
  },
): HomeCardData {
  if (context.authState !== "signed_in") {
    return {
      item,
      loading: false,
      facts: [
        "Sign in with Discord to load this server workspace.",
        "Home mirrors the navigation and opens the page directly.",
      ],
    };
  }

  if (!context.selectedGuildPresent) {
    return {
      item,
      loading: false,
      facts: [
        "Choose a server from the top bar to load this card.",
        "Each summary is scoped to the selected server.",
      ],
    };
  }

  switch (item.id) {
    case "control-panel":
      return buildControlPanelCard(item, context);
    case "stats":
      return buildStatsCard(item, context);
    case "commands":
      return buildCommandsCard(item, context);
    case "moderation":
      return buildAreaCard(item, context.moderationFeatures, context.workspaceState);
    case "logging":
      return buildAreaCard(item, context.loggingFeatures, context.workspaceState);
    case "partner-board":
      return buildPartnerBoardCard(item, context);
    case "autorole":
      return buildAutoroleCard(item, context);
    case "level-roles":
      return {
        item,
        loading: false,
        facts: [
          "This workspace is reserved for the future level-role table.",
          "The route already exists, but the editing flow is still pending.",
        ],
      };
    default:
      return {
        item,
        loading: false,
        facts: ["Open the page to review this workspace."],
      };
  }
}

function buildControlPanelCard(
  item: NavigationItem,
  context: Parameters<typeof buildHomeCardData>[1],
): HomeCardData {
  const readCount = context.rolesSettings.roles.dashboardReadRoleIds.length;
  const writeCount = context.rolesSettings.roles.dashboardWriteRoleIds.length;
  const loading =
    context.rolesSettings.loading && readCount === 0 && writeCount === 0;

  return {
    item,
    loading,
    facts: [
      `${formatCount(readCount, "read access role")}`,
      `${formatCount(writeCount, "write access role")}`,
      context.selectedGuildAccessLevel === "read"
        ? "Your current access to this server is read-only."
        : "Discord administrators keep implicit write access.",
    ],
  };
}

function buildStatsCard(
  item: NavigationItem,
  context: Parameters<typeof buildHomeCardData>[1],
): HomeCardData {
  if (context.workspaceState !== "ready" || context.statsFeature === null) {
    return {
      item,
      loading: context.workspaceState === "loading",
      facts: [summarizeWorkspaceGate(context.workspaceState, "Stats")],
    };
  }

  const statsDetails = getStatsFeatureDetails(context.statsFeature);
  return {
    item,
    loading: false,
    facts: [
      `${formatCount(statsDetails.configuredChannelCount, "configured channel")}`,
      `Update interval: ${statsDetails.updateIntervalMins} minute(s)`,
      summarizeStatsSignal(context.statsFeature),
    ],
  };
}

function buildCommandsCard(
  item: NavigationItem,
  context: Parameters<typeof buildHomeCardData>[1],
): HomeCardData {
  if (
    context.workspaceState !== "ready" ||
    context.commandsFeature === null ||
    context.adminCommandsFeature === null
  ) {
    return {
      item,
      loading: context.workspaceState === "loading",
      facts: [summarizeWorkspaceGate(context.workspaceState, "Commands")],
    };
  }

  const commandsDetails = getCommandsFeatureDetails(context.commandsFeature);
  const adminDetails = getAdminCommandsFeatureDetails(context.adminCommandsFeature);

  return {
    item,
    loading: false,
    facts: [
      commandsDetails.channelId === ""
        ? "Command channel not configured."
        : "Command channel configured.",
      `${formatCount(adminDetails.allowedRoleCount, "admin access role")}`,
      context.commandsFeature.blockers?.[0]?.message ??
        context.adminCommandsFeature.blockers?.[0]?.message ??
        "Command routing and access are available for review.",
    ],
  };
}

function buildAreaCard(
  item: NavigationItem,
  areaFeatures: FeatureRecord[],
  workspaceState: ReturnType<typeof useFeatureWorkspace>["workspaceState"],
): HomeCardData {
  if (workspaceState !== "ready") {
    return {
      item,
      loading: workspaceState === "loading",
      facts: [summarizeWorkspaceGate(workspaceState, item.label)],
    };
  }

  if (areaFeatures.length === 0) {
    return {
      item,
      loading: false,
      facts: ["No mapped controls are currently exposed for this area."],
    };
  }

  const areaSummary = summarizeFeatureArea(areaFeatures);
  const enabledCount = areaFeatures.filter((feature) => feature.effective_enabled).length;

  return {
    item,
    loading: false,
    facts: [
      `${enabledCount}/${areaFeatures.length} controls enabled`,
      areaSummary.label,
      areaSummary.signal,
    ],
  };
}

function buildPartnerBoardCard(
  item: NavigationItem,
  context: Parameters<typeof buildHomeCardData>[1],
): HomeCardData {
  const loading = context.partnerBoard.loading && context.partnerBoard.board === null;

  return {
    item,
    loading,
    facts: [
      `${formatCount(context.partnerBoard.partnerCount, "partner entry")}`,
      context.partnerBoard.deliveryConfigured
        ? context.partnerBoard.summarizePostingDestination
        : "Destination not configured.",
      context.partnerBoard.layoutConfigured
        ? "Layout configured."
        : "Layout still needs setup.",
    ],
  };
}

function buildAutoroleCard(
  item: NavigationItem,
  context: Parameters<typeof buildHomeCardData>[1],
): HomeCardData {
  if (context.workspaceState !== "ready" || context.autoRoleFeature === null) {
    return {
      item,
      loading: context.workspaceState === "loading",
      facts: [summarizeWorkspaceGate(context.workspaceState, "Autorole")],
    };
  }

  const details = getAutoRoleFeatureDetails(context.autoRoleFeature);
  return {
    item,
    loading: false,
    facts: [
      details.targetRoleId === "" ? "Target role not configured." : "Target role configured.",
      `${formatCount(details.requiredRoleCount, "required role")}`,
      summarizeAutoRoleSignal(context.autoRoleFeature),
    ],
  };
}

function summarizeWorkspaceGate(
  workspaceState: ReturnType<typeof useFeatureWorkspace>["workspaceState"],
  label: string,
) {
  switch (workspaceState) {
    case "checking":
      return "Checking your dashboard access for this area.";
    case "auth_required":
      return `Sign in with Discord before opening ${label.toLowerCase()}.`;
    case "server_required":
      return "Choose a server to load this area.";
    case "loading":
      return `Loading ${label.toLowerCase()} summary.`;
    case "unavailable":
      return `The ${label.toLowerCase()} summary could not be loaded.`;
    default:
      return `${label} is ready to open.`;
  }
}

function formatCount(count: number, label: string) {
  return `${count} ${label}${count === 1 ? "" : "s"}`;
}
