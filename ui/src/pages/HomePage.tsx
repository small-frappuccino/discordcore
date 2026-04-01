import { Link } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import {
  getDashboardHomeNavigationSections,
  type NavigationItem,
} from "../app/navigation";
import { FeatureWorkspaceLayout } from "../components/ui";
import {
  OverviewCard,
  OverviewStatRow,
  SectionBlock,
  type OverviewTone,
  type SemanticValueKind,
} from "../components/overview";
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
  sectionLabel: string;
  rows: HomeCardRow[];
  loading: boolean;
  state: OverviewTone;
}

interface HomeCardRow {
  label: string;
  value: string;
  valueKind: SemanticValueKind;
  state: OverviewTone;
}

export function HomePage() {
  const { authState, selectedGuild, selectedGuildID } = useDashboardSession();
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
  const activeGuildID = selectedGuildID?.trim() ?? "";
  const navigationSections =
    activeGuildID === "" ? [] : getDashboardHomeNavigationSections(activeGuildID);

  return (
    <section className="page-shell home-page">
      <h1 className="sr-only">Home</h1>

      <FeatureWorkspaceLayout
        notice={homeNotice}
        busyLabel={
          workspace.loading || rolesSettings.loading || partnerBoard.loading
            ? "Refreshing dashboard overview..."
            : undefined
        }
        workspaceEyebrow={null}
        workspaceTitle={null}
        workspaceDescription={null}
        workspaceClassName="home-workspace-panel"
        workspaceContent={
          navigationSections.length === 0 ? (
            <div className="table-empty-state table-empty-state-compact">
              <div className="card-copy">
                <p className="section-label">Workspace</p>
                <h2>
                  {authState === "signed_in" ? "Select a server" : "Sign in to continue"}
                </h2>
                <p className="section-description">
                  {authState === "signed_in"
                    ? "Choose a server from the top bar to open its management workspace."
                    : "Use Discord sign-in to load the servers you can manage here."}
                </p>
              </div>
            </div>
          ) : (
            <div className="home-category-stack">
              {navigationSections.map((section) => (
                <SectionBlock
                  className="home-category-section"
                  key={section.id}
                  eyebrow={null}
                  title={section.label}
                >
                  <div className="home-card-grid">
                    {section.items.map((item) => {
                      const card = buildHomeCardData(item, {
                        sectionLabel: section.label,
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
                        <OverviewCard
                          aria-busy={card.loading}
                          className={`home-nav-card${
                            item.id === "control-panel"
                              ? " home-nav-card-control-panel"
                              : ""
                          }`}
                          tone={card.state}
                          sectionLabel={null}
                          title={card.item.label}
                          action={
                            card.loading ? null : (
                              <Link className="button-secondary home-nav-link" to={card.item.to}>
                                {card.item.homeActionLabel ?? `Open ${card.item.label}`}
                              </Link>
                            )
                          }
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
                            <ul className="home-nav-facts overview-card-list">
                              {card.rows.map((row) => (
                                <OverviewStatRow
                                  key={`${row.label}-${row.value}`}
                                  label={row.label}
                                  value={row.value}
                                  kind={row.valueKind}
                                  tone={row.state}
                                  screenReaderLabel={`${row.label}: ${row.value}`}
                                />
                              ))}
                            </ul>
                          )}
                        </OverviewCard>
                      );
                    })}
                  </div>
                </SectionBlock>
              ))}
            </div>
          )
        }
      />
    </section>
  );
}

function buildHomeCardData(
  item: NavigationItem,
  context: {
    sectionLabel: string;
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
      context.sectionLabel,
      [createMetaCardRow("Status", "Loading", "attention")],
      true,
    );
  }

  if (context.authState !== "signed_in") {
    return createHomeCardData(item, context.sectionLabel, [
      createMetaCardRow("Status", "Sign in required", "attention"),
    ]);
  }

  if (!context.selectedGuildPresent) {
    return createHomeCardData(item, context.sectionLabel, [
      createMetaCardRow("Server", "Select a server", "attention"),
    ]);
  }

  switch (item.id) {
    case "control-panel":
      return buildControlPanelCard(item, context);
    case "stats":
      return buildStatsCard(item, context);
    case "commands":
      return buildCommandsCard(item, context);
    case "moderation":
      return buildFeatureToggleCard(
        item,
        context.sectionLabel,
        context.moderationFeatures,
        context.workspaceState,
      );
    case "logging":
      return buildFeatureToggleCard(
        item,
        context.sectionLabel,
        context.loggingFeatures,
        context.workspaceState,
      );
    case "partner-board":
      return buildPartnerBoardCard(item, context);
    case "autorole":
      return buildAutoroleCard(item, context);
    case "level-roles":
      return createHomeCardData(item, context.sectionLabel, [
        createMetaCardRow("Status", "In development", "attention"),
        createMetaCardRow("Availability", "Planned page"),
      ]);
    default:
      return createHomeCardData(item, context.sectionLabel, [
        createMetaCardRow("Status", "Ready"),
      ]);
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
    context.sectionLabel,
    [
      createNumericCardRow("Write roles", String(writeCount)),
      createNumericCardRow("Read roles", String(readCount)),
    ],
    loading,
    "enabled",
  );
}

function buildStatsCard(
  item: NavigationItem,
  context: Parameters<typeof buildHomeCardData>[1],
): HomeCardData {
  if (context.workspaceState !== "ready" || context.statsFeature === null) {
    return createHomeCardData(
      item,
      context.sectionLabel,
      [buildWorkspaceGateRow(context.workspaceState)],
      context.workspaceState === "loading",
    );
  }

  const statsDetails = getStatsFeatureDetails(context.statsFeature);
  return createHomeCardData(item, context.sectionLabel, [
    createFeatureStateRow("Module", context.statsFeature.effective_enabled),
    createNumericCardRow(
      "Configured channels",
      String(statsDetails.configuredChannelCount),
    ),
    createNumericCardRow("Update interval", `${statsDetails.updateIntervalMins} min`),
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
      context.sectionLabel,
      [buildWorkspaceGateRow(context.workspaceState)],
      context.workspaceState === "loading",
    );
  }

  const commandsDetails = getCommandsFeatureDetails(context.commandsFeature);
  const adminDetails = getAdminCommandsFeatureDetails(context.adminCommandsFeature);

  return createHomeCardData(item, context.sectionLabel, [
    createFeatureStateRow("Commands", context.commandsFeature.effective_enabled),
    createMetaCardRow(
      "Command channel",
      commandsDetails.channelId === "" ? "Missing" : "Configured",
      commandsDetails.channelId === "" ? "attention" : "neutral",
    ),
    createNumericCardRow("Admin roles", String(adminDetails.allowedRoleCount)),
  ]);
}

function buildFeatureToggleCard(
  item: NavigationItem,
  sectionLabel: string,
  areaFeatures: FeatureRecord[],
  workspaceState: ReturnType<typeof useFeatureWorkspace>["workspaceState"],
): HomeCardData {
  if (workspaceState !== "ready") {
    return createHomeCardData(
      item,
      sectionLabel,
      [buildWorkspaceGateRow(workspaceState)],
      workspaceState === "loading",
    );
  }

  if (areaFeatures.length === 0) {
    return createHomeCardData(item, sectionLabel, [
      createMetaCardRow("Status", "Not available", "attention"),
    ]);
  }

  return createHomeCardData(
    item,
    sectionLabel,
    areaFeatures.map((feature) =>
      createFeatureStateRow(feature.label, feature.effective_enabled),
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
    context.sectionLabel,
    [
      createNumericCardRow("Partners", String(context.partnerBoard.partnerCount)),
      createMetaCardRow(
        "Destination",
        context.partnerBoard.deliveryConfigured ? "Configured" : "Missing",
        context.partnerBoard.deliveryConfigured ? "neutral" : "attention",
      ),
      createMetaCardRow(
        "Layout",
        context.partnerBoard.layoutConfigured ? "Configured" : "Missing",
        context.partnerBoard.layoutConfigured ? "neutral" : "attention",
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
      context.sectionLabel,
      [buildWorkspaceGateRow(context.workspaceState)],
      context.workspaceState === "loading",
    );
  }

  const details = getAutoRoleFeatureDetails(context.autoRoleFeature);
  return createHomeCardData(item, context.sectionLabel, [
    createFeatureStateRow("Module", context.autoRoleFeature.effective_enabled),
    createMetaCardRow(
      "Target role",
      details.targetRoleId === "" ? "Missing" : "Configured",
      details.targetRoleId === "" ? "attention" : "neutral",
    ),
    createNumericCardRow("Required roles", String(details.requiredRoleCount)),
  ]);
}

function buildWorkspaceGateRow(
  workspaceState: ReturnType<typeof useFeatureWorkspace>["workspaceState"],
): HomeCardRow {
  switch (workspaceState) {
    case "checking":
      return createMetaCardRow("Status", "Checking access", "attention");
    case "auth_required":
      return createMetaCardRow("Status", "Sign in required", "attention");
    case "server_required":
      return createMetaCardRow("Server", "Select a server", "attention");
    case "loading":
      return createMetaCardRow("Status", "Loading", "attention");
    case "unavailable":
      return createMetaCardRow("Status", "Unavailable", "attention");
    default:
      return createMetaCardRow("Status", "Ready");
  }
}

function createHomeCardData(
  item: NavigationItem,
  sectionLabel: string,
  rows: HomeCardRow[],
  loading = false,
  stateOverride?: OverviewTone,
): HomeCardData {
  return {
    item,
    sectionLabel,
    rows,
    loading,
    state: stateOverride ?? summarizeHomeCardState(rows),
  };
}

function createFeatureStateRow(label: string, enabled: boolean): HomeCardRow {
  return {
    label,
    value: enabled ? "Enabled" : "Disabled",
    valueKind: "status",
    state: enabled ? "enabled" : "disabled",
  };
}

function createNumericCardRow(label: string, value: string): HomeCardRow {
  return {
    label,
    value,
    valueKind: "numeric",
    state: "neutral",
  };
}

function createMetaCardRow(
  label: string,
  value: string,
  state: OverviewTone = "neutral",
): HomeCardRow {
  return {
    label,
    value,
    valueKind: "meta",
    state,
  };
}

function summarizeHomeCardState(rows: HomeCardRow[]): OverviewTone {
  if (rows.some((row) => row.state === "disabled")) {
    return "disabled";
  }

  if (rows.some((row) => row.state === "attention")) {
    return "attention";
  }

  if (rows.some((row) => row.state === "enabled")) {
    return "enabled";
  }

  return "neutral";
}
