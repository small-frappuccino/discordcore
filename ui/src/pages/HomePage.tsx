import { Link } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import {
  dashboardHomeNavigationSections,
  type NavigationItem,
} from "../app/navigation";
import {
  AlertBanner,
  PageContentSurface,
  SurfaceCard,
} from "../components/ui";
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
  facts: HomeCardFact[];
  loading: boolean;
  tone: HomeFactTone;
}

type HomeFactTone = "neutral" | "info" | "success" | "error";
type HomeCardTier = "primary" | "secondary";

interface HomeCardFact {
  label: string;
  value: string;
  tone: HomeFactTone;
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
    <section className="page-shell home-page-shell">
      <header className="home-page-header">
        <h1>Home</h1>
      </header>

      <PageContentSurface>
        <AlertBanner notice={homeNotice} />

        <div className="home-category-stack">
          {dashboardHomeNavigationSections.map((section) => (
            <section className="home-category-section" key={section.id}>
              <div className="home-category-header">
                <h2>{section.label}</h2>
              </div>

              <div className="home-card-grid">
                {section.items.map((item, index) => {
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
                  const tier = getHomeCardTier(section.items.length, index);

                  return (
                    <SurfaceCard
                      as="article"
                      aria-busy={card.loading}
                      className={[
                        "home-nav-card",
                        "surface-card-accent",
                        card.tone !== "neutral"
                          ? `surface-card-accent-${card.tone}`
                          : "",
                        `home-nav-card-${tier}`,
                      ].join(" ")}
                      key={item.id}
                    >
                      {card.loading ? (
                        <div className="home-nav-card-skeleton" aria-hidden="true">
                          <span className="home-nav-skeleton home-nav-skeleton-title" />
                          <span className="home-nav-skeleton" />
                          <span className="home-nav-skeleton" />
                          <span className="home-nav-skeleton home-nav-skeleton-button" />
                        </div>
                      ) : (
                        <>
                          <div className="home-nav-card-header">
                            <h3>{card.item.label}</h3>
                          </div>

                          <ul className="home-nav-facts">
                            {card.facts.map((fact) => (
                              <li
                                className="home-nav-fact-row"
                                data-tone={fact.tone}
                                key={`${fact.label}:${fact.value}`}
                              >
                                <span className="sr-only">{`${fact.label}: ${fact.value}`}</span>
                                <div className="home-nav-fact-key" aria-hidden="true">
                                  <span className="home-nav-fact-label">{fact.label}: </span>
                                </div>
                                <div className="home-nav-fact-value-wrap" aria-hidden="true">
                                  {fact.tone !== "neutral" ? (
                                    <span
                                      className={`home-nav-fact-dot tone-${fact.tone}`}
                                      aria-hidden="true"
                                    />
                                  ) : null}
                                  <strong className="home-nav-fact-value">{fact.value}</strong>
                                </div>
                              </li>
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
      </PageContentSurface>
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
  if (context.authState === "checking") {
    return createHomeCardData(
      item,
      [createHomeCardFact("Status", "Loading")],
      true,
    );
  }

  if (context.authState !== "signed_in") {
    return createHomeCardData(item, [createHomeCardFact("Status", "Sign in required")]);
  }

  if (!context.selectedGuildPresent) {
    return createHomeCardData(item, [createHomeCardFact("Server", "Select a server")]);
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
      return createHomeCardData(item, [
        createHomeCardFact("Status", "In development"),
        createHomeCardFact("Focus", "Next page"),
      ]);
    default:
      return createHomeCardData(item, [createHomeCardFact("Status", "Ready")]);
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

  return createHomeCardData(
    item,
    [
      createHomeCardFact("Write roles", String(writeCount)),
      createHomeCardFact("Read roles", String(readCount)),
    ],
    loading,
  );
}

function buildStatsCard(
  item: NavigationItem,
  context: Parameters<typeof buildHomeCardData>[1],
): HomeCardData {
  if (context.workspaceState !== "ready" || context.statsFeature === null) {
    return createHomeCardData(
      item,
      [buildWorkspaceGateFact(context.workspaceState)],
      context.workspaceState === "loading",
    );
  }

  const statsDetails = getStatsFeatureDetails(context.statsFeature);
  return createHomeCardData(item, [
    createHomeCardFact("Configured channels", String(statsDetails.configuredChannelCount)),
    createHomeCardFact("Update interval", `${statsDetails.updateIntervalMins} min`),
    createHomeCardFact("Module", context.statsFeature.effective_enabled ? "On" : "Off"),
  ]);
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
    return createHomeCardData(
      item,
      [buildWorkspaceGateFact(context.workspaceState)],
      context.workspaceState === "loading",
    );
  }

  const commandsDetails = getCommandsFeatureDetails(context.commandsFeature);
  const adminDetails = getAdminCommandsFeatureDetails(context.adminCommandsFeature);

  return createHomeCardData(item, [
    createHomeCardFact("Commands", context.commandsFeature.effective_enabled ? "On" : "Off"),
    createHomeCardFact(
      "Command channel",
      commandsDetails.channelId === "" ? "Not configured" : "Configured",
    ),
    createHomeCardFact("Admin roles", String(adminDetails.allowedRoleCount)),
  ]);
}

function buildFeatureToggleCard(
  item: NavigationItem,
  areaFeatures: FeatureRecord[],
  workspaceState: ReturnType<typeof useFeatureWorkspace>["workspaceState"],
): HomeCardData {
  if (workspaceState !== "ready") {
    return createHomeCardData(
      item,
      [buildWorkspaceGateFact(workspaceState)],
      workspaceState === "loading",
    );
  }

  if (areaFeatures.length === 0) {
    return createHomeCardData(item, [createHomeCardFact("Status", "Not available")]);
  }

  return createHomeCardData(
    item,
    areaFeatures.map((feature) =>
      createHomeCardFact(feature.label, feature.effective_enabled ? "On" : "Off"),
    ),
  );
}

function buildPartnerBoardCard(
  item: NavigationItem,
  context: Parameters<typeof buildHomeCardData>[1],
): HomeCardData {
  const loading = context.partnerBoard.loading && context.partnerBoard.board === null;

  return createHomeCardData(
    item,
    [
      createHomeCardFact("Partners", String(context.partnerBoard.partnerCount)),
      createHomeCardFact(
        "Destination",
        context.partnerBoard.deliveryConfigured ? "Configured" : "Not configured",
      ),
      createHomeCardFact(
        "Layout",
        context.partnerBoard.layoutConfigured ? "Configured" : "Pending",
      ),
    ],
    loading,
  );
}

function buildAutoroleCard(
  item: NavigationItem,
  context: Parameters<typeof buildHomeCardData>[1],
): HomeCardData {
  if (context.workspaceState !== "ready" || context.autoRoleFeature === null) {
    return createHomeCardData(
      item,
      [buildWorkspaceGateFact(context.workspaceState)],
      context.workspaceState === "loading",
    );
  }

  const details = getAutoRoleFeatureDetails(context.autoRoleFeature);
  return createHomeCardData(item, [
    createHomeCardFact("Module", context.autoRoleFeature.effective_enabled ? "On" : "Off"),
    createHomeCardFact(
      "Target role",
      details.targetRoleId === "" ? "Not configured" : "Configured",
    ),
    createHomeCardFact("Required roles", String(details.requiredRoleCount)),
  ]);
}

function buildWorkspaceGateFact(
  workspaceState: ReturnType<typeof useFeatureWorkspace>["workspaceState"],
): HomeCardFact {
  switch (workspaceState) {
    case "checking":
      return createHomeCardFact("Status", "Checking access");
    case "auth_required":
      return createHomeCardFact("Status", "Sign in required");
    case "server_required":
      return createHomeCardFact("Server", "Select a server");
    case "loading":
      return createHomeCardFact("Status", "Loading");
    case "unavailable":
      return createHomeCardFact("Status", "Unavailable");
    default:
      return createHomeCardFact("Status", "Ready");
  }
}

function createHomeCardData(
  item: NavigationItem,
  facts: HomeCardFact[],
  loading = false,
): HomeCardData {
  return {
    item,
    facts,
    loading,
    tone: summarizeHomeCardTone(facts),
  };
}

function createHomeCardFact(label: string, value: string): HomeCardFact {
  return {
    label,
    value,
    tone: inferHomeFactTone(value),
  };
}

function inferHomeFactTone(value: string): HomeFactTone {
  const normalizedValue = value.trim().toLowerCase();

  if (
    normalizedValue === "on" ||
    normalizedValue === "configured" ||
    normalizedValue === "ready"
  ) {
    return "success";
  }

  if (
    normalizedValue === "off" ||
    normalizedValue === "not configured" ||
    normalizedValue === "sign in required" ||
    normalizedValue === "select a server" ||
    normalizedValue === "unavailable" ||
    normalizedValue === "not available"
  ) {
    return "error";
  }

  if (
    normalizedValue === "checking access" ||
    normalizedValue === "loading" ||
    normalizedValue === "pending" ||
    normalizedValue === "in development" ||
    normalizedValue === "next page"
  ) {
    return "info";
  }

  return "neutral";
}

function summarizeHomeCardTone(facts: HomeCardFact[]): HomeFactTone {
  if (facts.some((fact) => fact.tone === "error")) {
    return "error";
  }

  if (facts.some((fact) => fact.tone === "info")) {
    return "info";
  }

  if (facts.some((fact) => fact.tone === "success")) {
    return "success";
  }

  return "neutral";
}

function getHomeCardTier(itemCount: number, itemIndex: number): HomeCardTier {
  if (itemCount === 1 || itemIndex === 0) {
    return "primary";
  }

  return "secondary";
}
