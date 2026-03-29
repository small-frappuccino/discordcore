import { useEffect } from "react";
import { Link, useLocation } from "react-router-dom";
import { appRoutes } from "../app/routes";
import {
  formatAuthStateLabel,
  formatAuthSupportText,
  formatTimestamp,
} from "../app/utils";
import { KeyValueList, MetricCard, StatusBadge, SurfaceCard } from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { getFeatureAreaRecords, plannedModules } from "../features/features/areas";
import { summarizeFeatureArea, type FeatureStatusTone } from "../features/features/presentation";
import { getAutoRoleFeatureDetails, summarizeAutoRoleSignal } from "../features/features/roles";
import { getStatsFeatureDetails, summarizeStatsSignal } from "../features/features/stats";
import {
  useFeatureWorkspace,
  type FeatureWorkspaceState,
} from "../features/features/useFeatureWorkspace";
import { usePartnerBoardSummary } from "../features/partner-board/usePartnerBoardSummary";

type ModuleState = "on" | "off";
type ModuleReadiness = "ready" | "incomplete" | "blocked";

type HomeModuleSummary = {
  id: string;
  label: string;
  route: string;
  state: ModuleState;
  readiness: ModuleReadiness;
  signal: string;
  ctaLabel: string;
  ctaRoute: string;
  blockerAnchor?: string;
};

interface HomeModuleRow extends HomeModuleSummary {
  description: string;
  meta: string;
}

interface HomeBlocker {
  id: string;
  label: string;
  message: string;
  to: string;
  actionLabel: string;
}

export function HomePage() {
  const location = useLocation();
  const {
    authState,
    beginLogin,
    currentOriginLabel,
    manageableGuilds,
    selectedGuild,
    selectedGuildID,
    setSelectedGuildID,
  } = useDashboardSession();
  const featureWorkspace = useFeatureWorkspace({
    scope: "guild",
  });
  const partnerBoard = usePartnerBoardSummary();

  useEffect(() => {
    if (location.hash.trim() === "") {
      return;
    }

    const targetID = location.hash.replace(/^#/, "");
    requestAnimationFrame(() => {
      const target = document.getElementById(targetID);
      if (target && typeof target.scrollIntoView === "function") {
        target.scrollIntoView({
          block: "start",
        });
      }
    });
  }, [location.hash]);

  const selectedServerLabel = selectedGuild?.name ?? "No server selected";
  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const homeLoading = featureWorkspace.loading || partnerBoard.loading;
  const features = featureWorkspace.features;
  const commandsFeatures = getFeatureAreaRecords(features, "commands");
  const moderationFeatures = getFeatureAreaRecords(features, "moderation");
  const loggingFeatures = getFeatureAreaRecords(features, "logging");
  const autoRoleFeature =
    features.find((feature) => feature.id === "auto_role_assignment") ?? null;
  const statsFeature =
    features.find((feature) => feature.id === "stats_channels") ?? null;

  const modules: HomeModuleRow[] = [
    buildControlPanelModule(authState, selectedGuild !== null),
    buildPartnerBoardModule(authState, selectedGuild !== null, partnerBoard),
    buildFeatureAreaModule(
      "commands",
      "Commands",
      "Command routing and privileged command access stay in one dense workspace.",
      appRoutes.dashboardCoreCommands,
      commandsFeatures,
      featureWorkspace.workspaceState,
      commandsFeatures.length === 0
        ? "No mapped controls yet"
        : `${commandsFeatures.length} mapped controls`,
      "Open Commands",
    ),
    buildFeatureAreaModule(
      "moderation",
      "Moderation",
      "Mute role, AutoMod listener state, and staff enforcement routes stay visible inline.",
      appRoutes.dashboardModerationModeration,
      moderationFeatures,
      featureWorkspace.workspaceState,
      moderationFeatures.length === 0
        ? "No mapped controls yet"
        : `${moderationFeatures.length} mapped controls`,
      "Open Moderation",
    ),
    buildFeatureAreaModule(
      "logging",
      "Logging",
      "Operational log routes stay in one destination-first table with visible actions.",
      appRoutes.dashboardModerationLogging,
      loggingFeatures,
      featureWorkspace.workspaceState,
      loggingFeatures.length === 0
        ? "No mapped routes yet"
        : `${loggingFeatures.length} mapped routes`,
      "Open Logging",
    ),
    buildAutoroleModule(
      authState,
      selectedGuild !== null,
      autoRoleFeature,
      featureWorkspace.workspaceState,
    ),
    buildLevelRolesModule(authState, selectedGuild !== null),
    buildStatsModule(
      authState,
      selectedGuild !== null,
      statsFeature,
      featureWorkspace.workspaceState,
    ),
  ];

  const blockedModules = modules.filter((module) => module.readiness === "blocked");
  const incompleteModules = modules.filter(
    (module) => module.readiness === "incomplete",
  );
  const readyModules = modules.filter((module) => module.readiness === "ready");
  const blockers: HomeBlocker[] = blockedModules.map((module) => ({
    actionLabel: module.ctaLabel,
    id: module.id,
    label: module.label,
    message: module.signal,
    to: getModuleHref(module),
  }));
  const primaryActionModule =
    blockedModules[0] ??
    readyModules.find((module) => module.id === "partner-board") ??
    readyModules[0] ??
    incompleteModules[0] ??
    null;
  const moduleHealthTone =
    blockedModules.length > 0
      ? "error"
      : incompleteModules.length > 0
        ? "info"
        : "success";
  const summaryItems = [
    {
      className: "",
      description: formatAuthSupportText(authState, manageableGuilds.length),
      label: "Access state",
      tone: authState === "signed_in" ? "success" : "info",
      value:
        authState === "signed_in" ? "Connected" : formatAuthStateLabel(authState),
    },
    {
      className: "",
      description:
        selectedGuild === null
          ? "Choose the active server to load module readiness."
          : "The selected server drives the operational workspace.",
      label: "Server context",
      tone: selectedGuild === null ? "info" : "neutral",
      value: selectedServerLabel,
    },
    {
      className: "",
      description:
        blockedModules.length > 0
          ? `${blockedModules.length} blocked, ${incompleteModules.length} incomplete`
          : incompleteModules.length > 0
            ? `${incompleteModules.length} incomplete modules still need follow-up`
            : "Every listed module is currently ready",
      label: "Module health",
      tone: moduleHealthTone,
      value: `${readyModules.length}/${modules.length} ready`,
    },
    {
      className: "home-summary-card-blockers",
      description:
        blockedModules.length > 0
          ? blockedModules[0]!.signal
          : "No blocked modules are currently preventing use.",
      label: "Blockers",
      tone: blockedModules.length > 0 ? "error" : "success",
      value:
        blockedModules.length === 0
          ? "No active blockers"
          : `${blockedModules.length} active`,
    },
  ] as const;
  const quickShortcutItems = [
    {
      label: "Control Panel",
      value: (
        <Link className="button-secondary" to={appRoutes.dashboardCoreControlPanel}>
          Access roles
        </Link>
      ),
    },
    {
      label: "Partner Board",
      value: (
        <Link className="button-secondary" to={getModuleHref(modules[1]!)}>
          {modules[1]!.ctaLabel}
        </Link>
      ),
    },
    {
      label: "Commands",
      value: (
        <Link className="button-secondary" to={appRoutes.dashboardCoreCommands}>
          Command routing
        </Link>
      ),
    },
    {
      label: "Moderation",
      value: (
        <Link className="button-secondary" to={appRoutes.dashboardModerationModeration}>
          Enforcement setup
        </Link>
      ),
    },
  ];

  async function handleRefreshHome() {
    await Promise.all([featureWorkspace.refresh(), partnerBoard.refreshBoardSummary()]);
  }

  return (
    <section className="page-shell">
      <header className="page-header home-context-bar">
        <div className="home-context-top">
          <div className="home-context-identity">
            <div className="page-header-copy home-context-copy">
              <p className="page-eyebrow">Home</p>
              <div className="page-title-row">
                <h1>Home</h1>
                <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
                  {formatAuthStateLabel(authState)}
                </StatusBadge>
              </div>
              <p className="page-description">
                Central operational view for the primary Discordcore workspaces.
                Review readiness, act on blockers, and jump straight into the
                exact module that needs attention.
              </p>
            </div>

            <label className="field-stack home-context-server-field">
              <span className="field-label">Workspace server</span>
              <select
                aria-label="Workspace server"
                value={selectedGuildID}
                onChange={(event) => setSelectedGuildID(event.target.value)}
                disabled={authState !== "signed_in" || manageableGuilds.length === 0}
              >
                <option value="">
                  {authState !== "signed_in"
                    ? "Sign in to load servers"
                    : "Choose a server"}
                </option>
                {manageableGuilds.map((guild) => (
                  <option key={guild.id} value={guild.id}>
                    {guild.name}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <div className="page-actions home-context-actions">
            {authState === "signed_in" && selectedGuild !== null ? (
              <button
                className="button-secondary"
                type="button"
                disabled={homeLoading}
                onClick={() => void handleRefreshHome()}
              >
                Refresh home
              </button>
            ) : null}

            {authState !== "signed_in" ? (
              <button
                className="button-primary"
                type="button"
                onClick={() => void beginLogin(nextPath)}
              >
                Sign in with Discord
              </button>
            ) : selectedGuild === null ? (
              <span className="meta-note">
                Choose a server to unlock module shortcuts.
              </span>
            ) : primaryActionModule !== null ? (
              <Link className="button-primary" to={getModuleHref(primaryActionModule)}>
                {blockedModules.length > 0
                  ? `Resolve ${primaryActionModule.label}`
                  : primaryActionModule.ctaLabel}
              </Link>
            ) : null}
          </div>
        </div>

        <div className="page-meta home-context-meta">
          <span className="meta-pill subtle-pill">
            {authState === "signed_in" ? "Connected" : formatAuthStateLabel(authState)}
          </span>
          <span className="meta-pill subtle-pill">{selectedServerLabel}</span>
          <span className="meta-pill subtle-pill">{currentOriginLabel}</span>
        </div>
      </header>

      <section className="overview-summary-strip" aria-label="Home summary">
        {summaryItems.map((item) => (
          <MetricCard
            key={item.label}
            className={item.className}
            label={item.label}
            value={item.value}
            description={item.description}
            tone={item.tone}
          />
        ))}
      </section>

      <SurfaceCard className="home-area-card">
        <div className="home-area-card-header">
          <div className="card-copy">
            <p className="section-label">Workspace</p>
            <h2>Main modules</h2>
            <p className="section-description">
              Use the module table as the control index for every primary
              workspace. Status shows readiness, signal explains why, and the
              shortcut stays visible for direct follow-up.
            </p>
          </div>
          <StatusBadge tone={moduleHealthTone}>
            {blockedModules.length > 0
              ? `${blockedModules.length} blocked`
              : incompleteModules.length > 0
                ? `${incompleteModules.length} incomplete`
                : `${readyModules.length}/${modules.length} ready`}
          </StatusBadge>
        </div>

        <div className="table-wrap">
          <table className="data-table feature-table home-module-table">
            <thead>
              <tr>
                <th scope="col">Module</th>
                <th scope="col">Status</th>
                <th scope="col">Signal</th>
                <th scope="col">Shortcut</th>
              </tr>
            </thead>
            <tbody>
              {modules.map((module) => (
                <tr key={module.id}>
                  <td>
                    <div className="feature-table-copy">
                      <strong>{module.label}</strong>
                      <p>{module.description}</p>
                      <span className="meta-note">{module.meta}</span>
                    </div>
                  </td>
                  <td>
                    <div className="feature-status-cell home-module-status">
                      <StatusBadge tone={getReadinessTone(module.readiness)}>
                        {formatReadinessLabel(module.readiness)}
                      </StatusBadge>
                      <span className="meta-note">
                        State: {module.state === "on" ? "On" : "Off"}
                      </span>
                    </div>
                  </td>
                  <td>
                    <div className="feature-table-copy">
                      <p>{module.signal}</p>
                    </div>
                  </td>
                  <td>
                    <div className="feature-row-actions">
                      <Link className="button-secondary" to={getModuleHref(module)}>
                        {module.ctaLabel}
                      </Link>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </SurfaceCard>

      <section className="home-area-grid" aria-label="Home context">
        <SurfaceCard className="home-area-card home-blockers-card">
          <div className="home-area-card-header">
            <div className="card-copy">
              <p className="section-label">Current blockers</p>
              <h2>Current blockers</h2>
              <p className="section-description">
                Keep this list short and actionable. Every blocker links straight
                to the workspace or route that needs follow-up.
              </p>
            </div>
            <StatusBadge tone={blockedModules.length > 0 ? "error" : "success"}>
              {blockedModules.length === 0
                ? "No active blockers"
                : `${blockedModules.length} active`}
            </StatusBadge>
          </div>

          {authState !== "signed_in" ? (
            <p className="meta-note">
              Sign in with Discord before reviewing server blockers.
            </p>
          ) : selectedGuild === null ? (
            <p className="meta-note">
              Choose a server from the selector above before reviewing blockers.
            </p>
          ) : blockers.length === 0 ? (
            <p className="meta-note">
              No blocked modules are currently preventing use.
            </p>
          ) : (
            <ul className="feature-guidance-list">
              {blockers.map((blocker) => (
                <li key={blocker.id}>
                  <strong>{blocker.label}:</strong> {blocker.message}{" "}
                  <Link to={blocker.to}>{blocker.actionLabel}</Link>
                </li>
              ))}
            </ul>
          )}
        </SurfaceCard>

        <SurfaceCard className="home-area-card">
          <div className="home-area-card-header">
            <div className="card-copy">
              <p className="section-label">Quick shortcuts</p>
              <h2>Quick shortcuts</h2>
              <p className="section-description">
                Jump directly to the most common setup surfaces without scanning
                the full sidebar.
              </p>
            </div>
          </div>

          <KeyValueList className="workspace-status-list" items={[
            ...quickShortcutItems,
            {
              label: "Logging",
              value: (
                <Link className="button-secondary" to={appRoutes.dashboardModerationLogging}>
                  Log routes
                </Link>
              ),
            },
            {
              label: "Autorole",
              value: (
                <Link className="button-secondary" to={appRoutes.dashboardRolesAutorole}>
                  Automatic roles
                </Link>
              ),
            },
            {
              label: "Stats",
              value: (
                <Link className="button-secondary" to={appRoutes.dashboardCoreStats}>
                  Stats schedule
                </Link>
              ),
            },
            {
              label: "Diagnostics",
              value: (
                <Link className="button-secondary" to={`${appRoutes.settings}#diagnostics`}>
                  Open diagnostics
                </Link>
              ),
            },
          ]} />
        </SurfaceCard>

        <SurfaceCard className="home-area-card">
          <div className="home-area-card-header">
            <div className="card-copy">
              <p className="section-label">Secondary info</p>
              <h2>Settings and diagnostics</h2>
              <p className="section-description">
                Keep maintenance, diagnostics, and lower-frequency technical
                checks available without letting them dominate the landing page.
              </p>
            </div>
          </div>

          <p className="meta-note">
            Use Settings for connection, permissions, and diagnostics. Use
            Advanced only when you are working on maintenance routines.
          </p>

          <div className="home-area-footer">
            <Link className="button-secondary" to={appRoutes.settings}>
              Open Settings
            </Link>
            <Link className="button-secondary" to={`${appRoutes.settings}#diagnostics`}>
              Open Diagnostics
            </Link>
            <Link className="button-ghost" to={appRoutes.settingsAdvanced}>
              Open Advanced
            </Link>
          </div>
        </SurfaceCard>
      </section>

      <section
        className="home-planned-grid"
        id="planned"
        aria-label="Planned modules"
      >
        {plannedModules.map((module) => (
          <SurfaceCard
            className="roadmap-card roadmap-card-muted home-planned-card"
            key={module.id}
          >
            <div className="card-copy">
              <p className="section-label">Planned module</p>
              <h2>{module.label}</h2>
              <p className="section-description">{module.description}</p>
            </div>
            <div className="home-area-footer">
              <span className="meta-note">
                This stays out of the primary navigation until the workflow is
                intentionally designed.
              </span>
            </div>
          </SurfaceCard>
        ))}
      </section>
    </section>
  );
}

function buildControlPanelModule(
  authState: string,
  selectedGuildPresent: boolean,
): HomeModuleRow {
  if (authState !== "signed_in") {
    return {
      ctaLabel: "Open Control Panel",
      ctaRoute: appRoutes.dashboardCoreControlPanel,
      description:
        "Manage read and write dashboard access for the selected server.",
      id: "control-panel",
      label: "Control Panel",
      meta: "Read access • Write access",
      readiness: "incomplete",
      route: appRoutes.dashboardCoreControlPanel,
      signal: "Sign in with Discord before reviewing dashboard access roles.",
      state: "off",
    };
  }

  if (!selectedGuildPresent) {
    return {
      ctaLabel: "Open Control Panel",
      ctaRoute: appRoutes.dashboardCoreControlPanel,
      description:
        "Manage read and write dashboard access for the selected server.",
      id: "control-panel",
      label: "Control Panel",
      meta: "Read access • Write access",
      readiness: "incomplete",
      route: appRoutes.dashboardCoreControlPanel,
      signal: "Choose a server before reviewing read and write access roles.",
      state: "off",
    };
  }

  return {
    ctaLabel: "Open Control Panel",
    ctaRoute: appRoutes.dashboardCoreControlPanel,
    description:
      "Manage read and write dashboard access for the selected server.",
    id: "control-panel",
    label: "Control Panel",
    meta: "Read access • Write access",
    readiness: "ready",
    route: appRoutes.dashboardCoreControlPanel,
    signal:
      "Open the access workspace to review dashboard roles. Server admins remain implicitly allowed.",
    state: "on",
  };
}

function buildPartnerBoardModule(
  authState: string,
  selectedGuildPresent: boolean,
  partnerBoard: ReturnType<typeof usePartnerBoardSummary>,
): HomeModuleRow {
  if (authState !== "signed_in") {
    return {
      ctaLabel: "Open Partner Board",
      ctaRoute: appRoutes.partnerBoardEntries,
      description:
        "Entries, layout, and destination stay together in the publishing workspace.",
      id: "partner-board",
      label: "Partner Board",
      meta: "Publishing workspace",
      readiness: "incomplete",
      route: appRoutes.partnerBoardEntries,
      signal: "Sign in with Discord before loading partner board health.",
      state: "off",
    };
  }

  if (!selectedGuildPresent) {
    return {
      ctaLabel: "Open Partner Board",
      ctaRoute: appRoutes.partnerBoardEntries,
      description:
        "Entries, layout, and destination stay together in the publishing workspace.",
      id: "partner-board",
      label: "Partner Board",
      meta: "Publishing workspace",
      readiness: "incomplete",
      route: appRoutes.partnerBoardEntries,
      signal: "Choose a server before reviewing board layout, destination, and entries.",
      state: "off",
    };
  }

  const blocked =
    partnerBoard.shellStatus.tone === "error" ||
    !partnerBoard.deliveryConfigured ||
    !partnerBoard.layoutConfigured ||
    partnerBoard.partnerCount === 0;
  const noticeMessage = partnerBoard.notice?.message?.trim() ?? "";
  const signal =
    noticeMessage !== ""
      ? noticeMessage
      : !partnerBoard.deliveryConfigured
        ? partnerBoard.summarizePostingDestination
        : !partnerBoard.layoutConfigured
          ? "Board layout still needs to be completed before relying on the published output."
          : partnerBoard.partnerCount === 0
            ? "Add the first partner entry before relying on this board."
            : partnerBoard.shellStatus.description;

  return {
    ctaLabel:
      !partnerBoard.deliveryConfigured
        ? "Finish destination"
        : !partnerBoard.layoutConfigured
          ? "Review layout"
          : partnerBoard.partnerCount === 0
            ? "Manage entries"
            : "Open Partner Board",
    ctaRoute:
      !partnerBoard.deliveryConfigured
        ? appRoutes.partnerBoardDelivery
        : !partnerBoard.layoutConfigured
          ? appRoutes.partnerBoardLayout
          : appRoutes.partnerBoardEntries,
    description:
      "Entries, layout, and destination stay together in the publishing workspace.",
    id: "partner-board",
    label: "Partner Board",
    meta: `${partnerBoard.partnerCount} entries • ${partnerBoard.postingMethodLabel} • ${formatTimestamp(partnerBoard.lastLoadedAt, "Not checked yet")}`,
    readiness: blocked ? "blocked" : "ready",
    route: appRoutes.partnerBoardEntries,
    signal,
    state: "on",
  };
}

function buildFeatureAreaModule(
  id: string,
  label: string,
  description: string,
  route: string,
  areaFeatures: ReturnType<typeof getFeatureAreaRecords>,
  workspaceState: FeatureWorkspaceState,
  meta: string,
  ctaLabel: string,
): HomeModuleRow {
  if (workspaceState !== "ready") {
    return {
      ctaLabel,
      ctaRoute: route,
      description,
      id,
      label,
      meta,
      readiness: "incomplete",
      route,
      signal: summarizeWorkspaceGate(workspaceState, label),
      state: "off",
    };
  }

  if (areaFeatures.length === 0) {
    return {
      ctaLabel,
      ctaRoute: route,
      description,
      id,
      label,
      meta,
      readiness: "incomplete",
      route,
      signal: "This workspace does not expose any mapped controls for the selected server yet.",
      state: "off",
    };
  }

  const areaSummary = summarizeFeatureArea(areaFeatures);

  return {
    ctaLabel,
    ctaRoute: route,
    description,
    id,
    label,
    meta,
    readiness: getAreaReadiness(areaSummary),
    route,
    signal: areaSummary.signal,
    state: areaFeatures.some((feature) => feature.effective_enabled) ? "on" : "off",
  };
}

function buildAutoroleModule(
  authState: string,
  selectedGuildPresent: boolean,
  feature: ReturnType<typeof useFeatureWorkspace>["features"][number] | null,
  workspaceState: FeatureWorkspaceState,
): HomeModuleRow {
  if (workspaceState !== "ready" || authState !== "signed_in") {
    return {
      ctaLabel: "Open Autorole",
      ctaRoute: appRoutes.dashboardRolesAutorole,
      description:
        "Configure automatic role assignment with visible target and requirement controls.",
      id: "autorole",
      label: "Autorole",
      meta: "Target role • Requirement roles",
      readiness: "incomplete",
      route: appRoutes.dashboardRolesAutorole,
      signal: summarizeWorkspaceGate(workspaceState, "Autorole"),
      state: "off",
    };
  }

  if (!selectedGuildPresent || feature === null) {
    return {
      ctaLabel: "Open Autorole",
      ctaRoute: appRoutes.dashboardRolesAutorole,
      description:
        "Configure automatic role assignment with visible target and requirement controls.",
      id: "autorole",
      label: "Autorole",
      meta: "Target role • Requirement roles",
      readiness: "incomplete",
      route: appRoutes.dashboardRolesAutorole,
      signal:
        selectedGuildPresent
          ? "Automatic role assignment is not exposed for the selected server yet."
          : "Choose a server before reviewing automatic role assignment.",
      state: "off",
    };
  }

  const details = getAutoRoleFeatureDetails(feature);
  return {
    ctaLabel: "Open Autorole",
    ctaRoute: appRoutes.dashboardRolesAutorole,
    description:
      "Configure automatic role assignment with visible target and requirement controls.",
    id: "autorole",
    label: "Autorole",
    meta:
      details.requiredRoleCount === 0
        ? "No requirement roles selected"
        : `${details.requiredRoleCount} requirement roles`,
    readiness: mapFeatureReadiness(feature.readiness),
    route: appRoutes.dashboardRolesAutorole,
    signal: summarizeAutoRoleSignal(feature),
    state: feature.effective_enabled ? "on" : "off",
  };
}

function buildLevelRolesModule(
  authState: string,
  selectedGuildPresent: boolean,
): HomeModuleRow {
  if (authState !== "signed_in") {
    return {
      ctaLabel: "Open Level Roles",
      ctaRoute: appRoutes.dashboardRolesLevelRoles,
      description:
        "Manage the inline level-role table with one active rule at a time.",
      id: "level-roles",
      label: "Level Roles",
      meta: "Table editor",
      readiness: "incomplete",
      route: appRoutes.dashboardRolesLevelRoles,
      signal: "Sign in with Discord before reviewing level role entries.",
      state: "off",
    };
  }

  if (!selectedGuildPresent) {
    return {
      ctaLabel: "Open Level Roles",
      ctaRoute: appRoutes.dashboardRolesLevelRoles,
      description:
        "Manage the inline level-role table with one active rule at a time.",
      id: "level-roles",
      label: "Level Roles",
      meta: "Table editor",
      readiness: "incomplete",
      route: appRoutes.dashboardRolesLevelRoles,
      signal: "Choose a server before reviewing level role entries.",
      state: "off",
    };
  }

  return {
    ctaLabel: "Open Level Roles",
    ctaRoute: appRoutes.dashboardRolesLevelRoles,
    description:
      "Manage the inline level-role table with one active rule at a time.",
    id: "level-roles",
    label: "Level Roles",
    meta: "Table editor",
    readiness: "incomplete",
    route: appRoutes.dashboardRolesLevelRoles,
    signal:
      "Level role entries are not exposed by the control server yet. This route is reserved for the upcoming table refactor.",
    state: "off",
  };
}

function buildStatsModule(
  authState: string,
  selectedGuildPresent: boolean,
  feature: ReturnType<typeof useFeatureWorkspace>["features"][number] | null,
  workspaceState: FeatureWorkspaceState,
): HomeModuleRow {
  if (workspaceState !== "ready" || authState !== "signed_in") {
    return {
      ctaLabel: "Open Stats",
      ctaRoute: appRoutes.dashboardCoreStats,
      description:
        "Review the stats schedule and configured channel inventory for the selected server.",
      id: "stats",
      label: "Stats",
      meta: "Schedule • Channel inventory",
      readiness: "incomplete",
      route: appRoutes.dashboardCoreStats,
      signal: summarizeWorkspaceGate(workspaceState, "Stats"),
      state: "off",
    };
  }

  if (!selectedGuildPresent || feature === null) {
    return {
      ctaLabel: "Open Stats",
      ctaRoute: appRoutes.dashboardCoreStats,
      description:
        "Review the stats schedule and configured channel inventory for the selected server.",
      id: "stats",
      label: "Stats",
      meta: "Schedule • Channel inventory",
      readiness: "incomplete",
      route: appRoutes.dashboardCoreStats,
      signal:
        selectedGuildPresent
          ? "Stats channel controls are not exposed for the selected server yet."
          : "Choose a server before reviewing stats updates.",
      state: "off",
    };
  }

  const details = getStatsFeatureDetails(feature);
  return {
    ctaLabel: "Open Stats",
    ctaRoute: appRoutes.dashboardCoreStats,
    description:
      "Review the stats schedule and configured channel inventory for the selected server.",
    id: "stats",
    label: "Stats",
    meta:
      details.configuredChannelCount === 0
        ? "No configured channels"
        : `${details.configuredChannelCount} configured channels`,
    readiness: mapFeatureReadiness(feature.readiness),
    route: appRoutes.dashboardCoreStats,
    signal: summarizeStatsSignal(feature),
    state: feature.effective_enabled ? "on" : "off",
  };
}

function summarizeWorkspaceGate(
  workspaceState: FeatureWorkspaceState,
  label: string,
) {
  switch (workspaceState) {
    case "checking":
      return "Checking dashboard access before loading module readiness.";
    case "auth_required":
      return `Sign in with Discord before reviewing ${label.toLowerCase()}.`;
    case "server_required":
      return "Choose a server before loading server-scoped module health.";
    case "loading":
      return "Loading the latest module records for the selected server.";
    case "unavailable":
      return `The ${label.toLowerCase()} workspace could not be loaded for this server.`;
    default:
      return `${label} is ready to load.`;
  }
}

function mapFeatureReadiness(readiness: string): ModuleReadiness {
  if (readiness === "ready") {
    return "ready";
  }
  if (readiness === "blocked") {
    return "blocked";
  }
  return "incomplete";
}

function getAreaReadiness(
  areaSummary: ReturnType<typeof summarizeFeatureArea>,
): ModuleReadiness {
  if (areaSummary.blocked > 0) {
    return "blocked";
  }
  if (areaSummary.ready > 0) {
    return "ready";
  }
  return "incomplete";
}

function getModuleHref(module: HomeModuleSummary) {
  return module.blockerAnchor
    ? `${module.ctaRoute}${module.blockerAnchor}`
    : module.ctaRoute;
}

function formatReadinessLabel(readiness: ModuleReadiness) {
  if (readiness === "ready") {
    return "Ready";
  }
  if (readiness === "blocked") {
    return "Blocked";
  }
  return "Incomplete";
}

function getReadinessTone(readiness: ModuleReadiness): FeatureStatusTone {
  if (readiness === "ready") {
    return "success";
  }
  if (readiness === "blocked") {
    return "error";
  }
  return "info";
}
