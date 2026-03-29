import { Link } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import {
  dashboardHomeNavigationSections,
  type NavigationItem,
} from "../app/navigation";
import { AlertBanner, SurfaceCard } from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { useGuildRolesSettings } from "../features/control-panel/useGuildRolesSettings";
import {
  getAdminCommandsFeatureDetails,
  getCommandsFeatureDetails,
} from "../features/features/commands";
import { getFeatureAreaRecords } from "../features/features/areas";
import { getAutoRoleFeatureDetails } from "../features/features/roles";
import { getStatsFeatureDetails } from "../features/features/stats";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { usePartnerBoardSummary } from "../features/partner-board/usePartnerBoardSummary";

interface HomeCardData {
  item: NavigationItem;
  facts: string[];
  loading: boolean;
}

export function HomePage() {
  const { authState, selectedGuild } = useDashboardSession();
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

  return (
    <section className="page-shell">
      <header className="home-page-header">
        <h1>Home</h1>
      </header>

      <AlertBanner notice={homeNotice} />

      <div className="home-category-stack">
        {dashboardHomeNavigationSections.map((section) => (
          <section className="home-category-section" key={section.id}>
            <div className="home-category-header">
              <h2>{section.label}</h2>
            </div>

            <div className="home-card-grid">
              {section.items.map((item) => {
                const card = buildHomeCardData(item, {
                  authState,
                  selectedGuildPresent: selectedGuild !== null,
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
                          <h3>{card.item.label}</h3>
                        </div>

                        <ul className="home-nav-facts">
                          {card.facts.map((fact) => (
                            <li key={fact}>{fact}</li>
                          ))}
                        </ul>

                        <div className="home-nav-card-footer">
                          <Link className="button-secondary home-nav-link" to={card.item.to}>
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
      facts: ["Status: Sign in required"],
    };
  }

  if (!context.selectedGuildPresent) {
    return {
      item,
      loading: false,
      facts: ["Server: Select a server"],
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
      return buildFeatureToggleCard(item, context.moderationFeatures, context.workspaceState);
    case "logging":
      return buildFeatureToggleCard(item, context.loggingFeatures, context.workspaceState);
    case "partner-board":
      return buildPartnerBoardCard(item, context);
    case "autorole":
      return buildAutoroleCard(item, context);
    case "level-roles":
      return {
        item,
        loading: false,
        facts: ["Status: In development", "Focus: Next page"],
      };
    default:
      return {
        item,
        loading: false,
        facts: ["Status: Ready"],
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
    facts: [`Write roles: ${writeCount}`, `Read roles: ${readCount}`],
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
      facts: [summarizeWorkspaceGate(context.workspaceState)],
    };
  }

  const statsDetails = getStatsFeatureDetails(context.statsFeature);
  return {
    item,
    loading: false,
    facts: [
      `Configured channels: ${statsDetails.configuredChannelCount}`,
      `Update interval: ${statsDetails.updateIntervalMins} min`,
      `Module: ${context.statsFeature.effective_enabled ? "On" : "Off"}`,
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
      facts: [summarizeWorkspaceGate(context.workspaceState)],
    };
  }

  const commandsDetails = getCommandsFeatureDetails(context.commandsFeature);
  const adminDetails = getAdminCommandsFeatureDetails(context.adminCommandsFeature);

  return {
    item,
    loading: false,
    facts: [
      `Commands: ${context.commandsFeature.effective_enabled ? "On" : "Off"}`,
      `Command channel: ${commandsDetails.channelId === "" ? "Not configured" : "Configured"}`,
      `Admin roles: ${adminDetails.allowedRoleCount}`,
    ],
  };
}

function buildFeatureToggleCard(
  item: NavigationItem,
  areaFeatures: FeatureRecord[],
  workspaceState: ReturnType<typeof useFeatureWorkspace>["workspaceState"],
): HomeCardData {
  if (workspaceState !== "ready") {
    return {
      item,
      loading: workspaceState === "loading",
      facts: [summarizeWorkspaceGate(workspaceState)],
    };
  }

  if (areaFeatures.length === 0) {
    return {
      item,
      loading: false,
      facts: ["Status: Not available"],
    };
  }

  return {
    item,
    loading: false,
    facts: areaFeatures.map(
      (feature) => `${feature.label}: ${feature.effective_enabled ? "On" : "Off"}`,
    ),
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
      `Partners: ${context.partnerBoard.partnerCount}`,
      `Destination: ${context.partnerBoard.deliveryConfigured ? "Configured" : "Not configured"}`,
      `Layout: ${context.partnerBoard.layoutConfigured ? "Configured" : "Pending"}`,
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
      facts: [summarizeWorkspaceGate(context.workspaceState)],
    };
  }

  const details = getAutoRoleFeatureDetails(context.autoRoleFeature);
  return {
    item,
    loading: false,
    facts: [
      `Module: ${context.autoRoleFeature.effective_enabled ? "On" : "Off"}`,
      `Target role: ${details.targetRoleId === "" ? "Not configured" : "Configured"}`,
      `Required roles: ${details.requiredRoleCount}`,
    ],
  };
}

function summarizeWorkspaceGate(
  workspaceState: ReturnType<typeof useFeatureWorkspace>["workspaceState"],
) {
  switch (workspaceState) {
    case "checking":
      return "Status: Checking access";
    case "auth_required":
      return "Status: Sign in required";
    case "server_required":
      return "Server: Select a server";
    case "loading":
      return "Status: Loading";
    case "unavailable":
      return "Status: Unavailable";
    default:
      return "Status: Ready";
  }
}
