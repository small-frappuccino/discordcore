import { useEffect } from "react";
import { Link, useLocation } from "react-router-dom";
import { appRoutes, getFeatureAreaRoute } from "../app/routes";
import {
  formatAuthStateLabel,
  formatAuthSupportText,
  formatTimestamp,
} from "../app/utils";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  KeyValueList,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../components/ui";
import {
  getFeatureAreaRecords,
  plannedModules,
  primaryFeatureAreaDefinitions,
} from "../features/features/areas";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { usePartnerBoardSummary } from "../features/partner-board/usePartnerBoardSummary";

type WorkspaceState = ReturnType<typeof useFeatureWorkspace>["workspaceState"];

interface HomeModuleSummary {
  id: string;
  label: string;
  description: string;
  meta: string;
  tone: "neutral" | "info" | "success" | "error";
  statusLabel: string;
  signal: string;
  primaryTo: string;
  primaryLabel: string;
  secondaryTo?: string;
  secondaryLabel?: string;
  blocked: boolean;
}

interface HomeBlocker {
  id: string;
  label: string;
  message: string;
  to: string;
  actionLabel: string;
}

type HomeShortcutItems = Parameters<typeof KeyValueList>[0]["items"];

export function HomePage() {
  const location = useLocation();
  const {
    authState,
    beginLogin,
    currentOriginLabel,
    manageableGuilds,
    selectedGuild,
  } = useDashboardSession();
  const featureWorkspace = useFeatureWorkspace({
    scope: "guild",
  });
  const {
    deliveryConfigured,
    lastLoadedAt,
    layoutConfigured,
    loading: boardSummaryLoading,
    partnerCount,
    postingMethodLabel,
    refreshBoardSummary,
    shellStatus,
    summarizePostingDestination,
  } = usePartnerBoardSummary();

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
  const homeLoading = featureWorkspace.loading || boardSummaryLoading;
  const featureAreaModules = primaryFeatureAreaDefinitions.map((area) => {
    const areaSummary = summarizeFeatureArea(
      getFeatureAreaRecords(featureWorkspace.features, area.id),
      featureWorkspace.workspaceState,
    );

    return {
      area,
      summary: areaSummary,
    };
  });

  const partnerBoardModule = buildPartnerBoardModule({
    authState,
    deliveryConfigured,
    lastLoadedAt,
    layoutConfigured,
    partnerCount,
    postingMethodLabel,
    selectedGuildPresent: selectedGuild !== null,
    shellStatus,
    summarizePostingDestination,
  });
  const mainModules: HomeModuleSummary[] = [
    partnerBoardModule,
    ...featureAreaModules.map(({ area, summary }) => ({
      blocked: summary.tone === "error",
      description: area.description,
      id: area.id,
      label: area.label,
      meta:
        summary.total === 0
          ? "No mapped features yet"
          : `${summary.total} tracked features`,
      primaryLabel: `Open ${area.label}`,
      primaryTo: getFeatureAreaRoute(area.id),
      signal: summary.support,
      statusLabel: summary.label,
      tone: summary.tone,
    })),
  ];
  const blockedModules = mainModules.filter((module) => module.blocked);
  const readyModules = mainModules.filter(
    (module) => module.tone === "success",
  );
  const blockers = blockedModules.map(
    (module): HomeBlocker => ({
      actionLabel: module.secondaryLabel ?? module.primaryLabel,
      id: module.id,
      label: module.label,
      message: module.signal,
      to: module.secondaryTo ?? module.primaryTo,
    }),
  );
  const quickShortcutItems = [
    {
      label: "Partner Board",
      value: (
        <Link className="button-secondary" to={appRoutes.partnerBoardEntries}>
          Board entries
        </Link>
      ),
    },
    ...primaryFeatureAreaDefinitions.map((area) => ({
      label: area.label,
      value: (
        <Link className="button-secondary" to={getFeatureAreaRoute(area.id)}>
          {area.homeShortcutLabel}
        </Link>
      ),
    })),
    {
      label: "Settings",
      value: (
        <Link className="button-secondary" to={appRoutes.settings}>
          Diagnostics
        </Link>
      ),
    },
  ];
  const homeStatusTone = getHomeOperationalTone(
    authState,
    selectedGuild !== null,
    blockers.length,
  );
  const mainModulesLabel =
    authState !== "signed_in"
      ? "Sign in required"
      : selectedGuild === null
        ? "Choose a server"
        : `${readyModules.length}/${mainModules.length} operational`;
  const mainModulesSupport =
    authState !== "signed_in"
      ? formatAuthSupportText(authState, manageableGuilds.length)
      : selectedGuild === null
        ? "Choose a server to load module health and blockers."
        : `${blockers.length} blockers across the main modules.`;
  const blockersLabel =
    authState !== "signed_in"
      ? "Waiting for access"
      : selectedGuild === null
        ? "Choose a server"
        : blockers.length === 0
          ? "All clear"
          : `${blockers.length} active`;
  const blockersSupport =
    authState !== "signed_in"
      ? "Sign in before reviewing module blockers."
      : selectedGuild === null
        ? "Choose a server to inspect blockers."
        : (blockers[0]?.message ??
          "No active blockers across the main modules.");
  const summaryItems = [
    {
      label: "Access",
      value: formatAuthStateLabel(authState),
      description: formatAuthSupportText(authState, manageableGuilds.length),
      tone: authState === "signed_in" ? "success" : "info",
    },
    {
      label: "Server",
      value: selectedServerLabel,
      description:
        selectedGuild === null
          ? "Choose a server to load category health and blocker summaries."
          : "The selected server drives every server-scoped workspace.",
      tone: selectedGuild === null ? "info" : "neutral",
    },
    {
      label: "Main modules",
      value: mainModulesLabel,
      description: mainModulesSupport,
      tone:
        authState === "signed_in" && selectedGuild !== null
          ? blockers.length > 0
            ? "error"
            : "success"
          : "info",
    },
    {
      label: "Blockers",
      value: blockersLabel,
      description: blockersSupport,
      tone:
        authState === "signed_in" && selectedGuild !== null
          ? blockers.length > 0
            ? "error"
            : "success"
          : "info",
    },
  ] as const;

  async function handleRefreshHome() {
    await Promise.all([featureWorkspace.refresh(), refreshBoardSummary()]);
  }

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Home"
        title="Home"
        description="See the main modules first, review blockers, and jump straight into the right workspace."
        status={
          <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
            {formatAuthStateLabel(authState)}
          </StatusBadge>
        }
        meta={
          <>
            <span className="meta-pill subtle-pill">{selectedServerLabel}</span>
            <span className="meta-pill subtle-pill">{currentOriginLabel}</span>
          </>
        }
        actions={
          <HomeHeaderActions
            authState={authState}
            homeLoading={homeLoading}
            selectedGuildPresent={selectedGuild !== null}
            onLogin={() => void beginLogin(nextPath)}
            onRefresh={() => void handleRefreshHome()}
          />
        }
      />

      <section className="overview-summary-strip" aria-label="Home summary">
        {summaryItems.map((item) => (
          <MetricCard
            key={item.label}
            label={item.label}
            value={item.value}
            description={item.description}
            tone={item.tone}
          />
        ))}
      </section>

      <HomeMainModulesCard
        mainModules={mainModules}
        mainModulesLabel={mainModulesLabel}
        statusTone={homeStatusTone}
      />

      <HomeContextCards
        authState={authState}
        blockers={blockers}
        blockersLabel={blockersLabel}
        homeLoading={homeLoading}
        quickShortcutItems={quickShortcutItems}
        selectedGuildPresent={selectedGuild !== null}
        statusTone={homeStatusTone}
        onRefresh={() => void handleRefreshHome()}
      />

      <HomePlannedModulesSection />
    </section>
  );
}

interface HomeHeaderActionsProps {
  authState: string;
  homeLoading: boolean;
  selectedGuildPresent: boolean;
  onLogin: () => void;
  onRefresh: () => void;
}

function HomeHeaderActions({
  authState,
  homeLoading,
  selectedGuildPresent,
  onLogin,
  onRefresh,
}: HomeHeaderActionsProps) {
  return (
    <>
      {authState === "signed_in" && selectedGuildPresent ? (
        <button
          className="button-secondary"
          type="button"
          disabled={homeLoading}
          onClick={onRefresh}
        >
          Refresh home
        </button>
      ) : null}

      {authState !== "signed_in" ? (
        <button className="button-primary" type="button" onClick={onLogin}>
          Sign in with Discord
        </button>
      ) : !selectedGuildPresent ? (
        <span className="meta-note">
          Choose a server from the sidebar to load server-scoped categories.
        </span>
      ) : (
        <Link className="button-primary" to={appRoutes.partnerBoardEntries}>
          Open Partner Board
        </Link>
      )}
    </>
  );
}

interface HomeMainModulesCardProps {
  mainModules: HomeModuleSummary[];
  mainModulesLabel: string;
  statusTone: HomeModuleSummary["tone"];
}

function HomeMainModulesCard({
  mainModules,
  mainModulesLabel,
  statusTone,
}: HomeMainModulesCardProps) {
  return (
    <SurfaceCard className="home-area-card">
      <div className="home-area-card-header">
        <div className="card-copy">
          <p className="section-label">Workspace</p>
          <h2>Main modules</h2>
          <p className="section-description">
            Start from the main operator workspaces and use the signal column
            to decide which module needs attention first.
          </p>
        </div>
        <StatusBadge tone={statusTone}>{mainModulesLabel}</StatusBadge>
      </div>

      <div className="table-wrap">
        <table className="data-table feature-table">
          <thead>
            <tr>
              <th scope="col">Module</th>
              <th scope="col">Status</th>
              <th scope="col">Signal</th>
              <th scope="col">Shortcut</th>
            </tr>
          </thead>
          <tbody>
            {mainModules.map((module) => (
              <tr key={module.id}>
                <td>
                  <div className="feature-table-copy">
                    <strong>{module.label}</strong>
                    <p>{module.description}</p>
                    <span className="meta-note">{module.meta}</span>
                  </div>
                </td>
                <td>
                  <div className="feature-status-cell">
                    <StatusBadge tone={module.tone}>{module.statusLabel}</StatusBadge>
                  </div>
                </td>
                <td>
                  <div className="feature-table-copy">
                    <p>{module.signal}</p>
                  </div>
                </td>
                <td>
                  <div className="feature-row-actions">
                    <Link className="button-secondary" to={module.primaryTo}>
                      {module.primaryLabel}
                    </Link>
                    {module.secondaryTo && module.secondaryLabel ? (
                      <Link className="button-ghost" to={module.secondaryTo}>
                        {module.secondaryLabel}
                      </Link>
                    ) : null}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </SurfaceCard>
  );
}

interface HomeContextCardsProps {
  authState: string;
  blockers: HomeBlocker[];
  blockersLabel: string;
  homeLoading: boolean;
  quickShortcutItems: HomeShortcutItems;
  selectedGuildPresent: boolean;
  statusTone: HomeModuleSummary["tone"];
  onRefresh: () => void;
}

function HomeContextCards({
  authState,
  blockers,
  blockersLabel,
  homeLoading,
  quickShortcutItems,
  selectedGuildPresent,
  statusTone,
  onRefresh,
}: HomeContextCardsProps) {
  return (
    <section className="home-area-grid" aria-label="Home context">
      <HomeBlockersCard
        authState={authState}
        blockers={blockers}
        blockersLabel={blockersLabel}
        homeLoading={homeLoading}
        selectedGuildPresent={selectedGuildPresent}
        statusTone={statusTone}
        onRefresh={onRefresh}
      />
      <HomeQuickShortcutsCard items={quickShortcutItems} />
      <HomeSecondaryCard />
    </section>
  );
}

interface HomeBlockersCardProps {
  authState: string;
  blockers: HomeBlocker[];
  blockersLabel: string;
  homeLoading: boolean;
  selectedGuildPresent: boolean;
  statusTone: HomeModuleSummary["tone"];
  onRefresh: () => void;
}

function HomeBlockersCard({
  authState,
  blockers,
  blockersLabel,
  homeLoading,
  selectedGuildPresent,
  statusTone,
  onRefresh,
}: HomeBlockersCardProps) {
  return (
    <SurfaceCard className="home-area-card">
      <div className="home-area-card-header">
        <div className="card-copy">
          <p className="section-label">Focus</p>
          <h2>Current blockers</h2>
          <p className="section-description">
            Keep the blocker list short and actionable so setup work starts
            with the modules that are actually preventing use.
          </p>
        </div>
        <StatusBadge tone={statusTone}>{blockersLabel}</StatusBadge>
      </div>

      {authState !== "signed_in" ? (
        <p className="meta-note">
          Sign in with Discord before reviewing server blockers.
        </p>
      ) : !selectedGuildPresent ? (
        <p className="meta-note">
          Choose a server from the sidebar before reviewing blockers.
        </p>
      ) : blockers.length === 0 ? (
        <p className="meta-note">
          No active blockers across Partner Board, commands, moderation,
          logging, roles, or stats.
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

      {authState === "signed_in" && selectedGuildPresent ? (
        <div className="home-area-footer">
          <button
            className="button-secondary"
            type="button"
            disabled={homeLoading}
            onClick={onRefresh}
          >
            Refresh blockers
          </button>
        </div>
      ) : null}
    </SurfaceCard>
  );
}

interface HomeQuickShortcutsCardProps {
  items: HomeShortcutItems;
}

function HomeQuickShortcutsCard({ items }: HomeQuickShortcutsCardProps) {
  return (
    <SurfaceCard className="home-area-card">
      <div className="home-area-card-header">
        <div className="card-copy">
          <p className="section-label">Shortcuts</p>
          <h2>Quick shortcuts</h2>
          <p className="section-description">
            Jump straight into the most common setup tasks without scanning
            the full navigation every time.
          </p>
        </div>
      </div>

      <KeyValueList className="workspace-status-list" items={items} />
    </SurfaceCard>
  );
}

function HomeSecondaryCard() {
  return (
    <SurfaceCard className="home-area-card">
      <div className="home-area-card-header">
        <div className="card-copy">
          <p className="section-label">Secondary</p>
          <h2>Advanced stays in Settings</h2>
          <p className="section-description">
            Cleanup, backfill, prune, cache, and diagnostics remain outside
            the main Home flow so they stay available without dominating the
            landing page.
          </p>
        </div>
      </div>

      <p className="meta-note">
        Use Settings for diagnostics and connection work. Use Settings &gt;
        Advanced only when you are working on maintenance routines.
      </p>

      <div className="home-area-footer">
        <Link className="button-secondary" to={appRoutes.settings}>
          Open settings
        </Link>
        <Link className="button-secondary" to={appRoutes.settingsAdvanced}>
          Open Settings &gt; Advanced
        </Link>
      </div>
    </SurfaceCard>
  );
}

function HomePlannedModulesSection() {
  return (
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
              This stays off the main navigation until the operator workflow
              is intentionally designed.
            </span>
          </div>
        </SurfaceCard>
      ))}
    </section>
  );
}

function getHomeOperationalTone(
  authState: string,
  selectedGuildPresent: boolean,
  blockerCount: number,
): HomeModuleSummary["tone"] {
  if (authState !== "signed_in" || !selectedGuildPresent) {
    return "info";
  }

  return blockerCount > 0 ? "error" : "success";
}

function buildPartnerBoardModule({
  authState,
  deliveryConfigured,
  lastLoadedAt,
  layoutConfigured,
  partnerCount,
  postingMethodLabel,
  selectedGuildPresent,
  shellStatus,
  summarizePostingDestination,
}: {
  authState: string;
  deliveryConfigured: boolean;
  lastLoadedAt: number | null;
  layoutConfigured: boolean;
  partnerCount: number;
  postingMethodLabel: string;
  selectedGuildPresent: boolean;
  shellStatus: ReturnType<typeof usePartnerBoardSummary>["shellStatus"];
  summarizePostingDestination: string;
}): HomeModuleSummary {
  if (authState !== "signed_in") {
    return {
      blocked: false,
      description:
        "Entries, layout, and delivery stay together in the dedicated publishing workspace.",
      id: "partner-board",
      label: "Partner Board",
      meta: "Publishing workspace",
      primaryLabel: "Open Partner Board",
      primaryTo: appRoutes.partnerBoardEntries,
      signal: "Sign in with Discord to load the Partner Board workspace.",
      statusLabel: "Sign in required",
      tone: "info",
    };
  }

  if (!selectedGuildPresent) {
    return {
      blocked: false,
      description:
        "Entries, layout, and delivery stay together in the dedicated publishing workspace.",
      id: "partner-board",
      label: "Partner Board",
      meta: "Choose a server",
      primaryLabel: "Open Partner Board",
      primaryTo: appRoutes.partnerBoardEntries,
      signal: "Choose a server to review destination, layout, and entries.",
      statusLabel: "Choose a server",
      tone: "info",
    };
  }

  const blocked =
    shellStatus.tone === "error" || !deliveryConfigured || !layoutConfigured;

  return {
    blocked,
    description:
      "Entries, layout, and delivery stay together in the dedicated publishing workspace.",
    id: "partner-board",
    label: "Partner Board",
    meta: `${partnerCount} entries • ${postingMethodLabel} • ${formatTimestamp(lastLoadedAt, "Not checked yet")}`,
    primaryLabel: "Open Partner Board",
    primaryTo: appRoutes.partnerBoardEntries,
    secondaryLabel: !deliveryConfigured
      ? "Finish destination"
      : !layoutConfigured
        ? "Review layout"
        : undefined,
    secondaryTo: !deliveryConfigured
      ? appRoutes.partnerBoardDelivery
      : !layoutConfigured
        ? appRoutes.partnerBoardLayout
        : undefined,
    signal: !deliveryConfigured
      ? summarizePostingDestination
      : !layoutConfigured
        ? "Board layout still needs to be reviewed before relying on the published output."
        : shellStatus.description,
    statusLabel: shellStatus.label,
    tone: shellStatus.tone,
  };
}

function formatHomeWorkspaceLabel(state: WorkspaceState) {
  switch (state) {
    case "checking":
      return "Checking access";
    case "auth_required":
      return "Sign in required";
    case "server_required":
      return "Choose a server";
    case "loading":
      return "Loading modules";
    case "unavailable":
      return "Unavailable";
    case "ready":
      return "Ready";
    default:
      return "Unavailable";
  }
}

function formatHomeWorkspaceSupport(state: WorkspaceState) {
  switch (state) {
    case "checking":
      return "The dashboard is verifying session access.";
    case "auth_required":
      return "Sign in with Discord to load the main modules.";
    case "server_required":
      return "Choose a server to load guild feature readiness.";
    case "loading":
      return "Loading the latest feature records for the selected server.";
    case "unavailable":
      return "The feature workspace could not be loaded for this server.";
    case "ready":
      return "Main modules are loaded and ready to summarize.";
    default:
      return "Main modules are unavailable.";
  }
}

function summarizeFeatureArea(
  features: ReturnType<typeof getFeatureAreaRecords>,
  workspaceState: WorkspaceState,
) {
  if (workspaceState !== "ready") {
    return {
      blocked: 0,
      label: formatHomeWorkspaceLabel(workspaceState),
      ready: 0,
      support: formatHomeWorkspaceSupport(workspaceState),
      tone: "info" as const,
      total: 0,
    };
  }

  const total = features.length;
  const ready = features.filter(
    (feature) => feature.readiness === "ready",
  ).length;
  const blocked = features.filter(
    (feature) => feature.readiness === "blocked",
  ).length;
  const disabled = features.filter(
    (feature) => !feature.effective_enabled,
  ).length;

  let tone: "neutral" | "info" | "success" | "error" = "neutral";
  let label = "Disabled";
  let support =
    total === 0
      ? "No feature records are mapped to this module yet."
      : "Everything is currently disabled.";

  if (blocked > 0) {
    tone = "error";
    label = "Needs attention";
    support =
      features.find((feature) => feature.blockers?.length)?.blockers?.[0]
        ?.message ?? "One or more features in this module are blocked.";
  } else if (ready > 0) {
    tone = "success";
    label = "Operational";
    support =
      disabled === 0
        ? "Everything mapped to this module is ready."
        : "At least one feature in this module is ready to use.";
  }

  return {
    blocked,
    label,
    ready,
    support,
    tone,
    total,
  };
}
