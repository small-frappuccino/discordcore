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
import { featureAreaDefinitions, getFeatureAreaRecords, plannedModules } from "../features/features/areas";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { usePartnerBoardSummary } from "../features/partner-board/usePartnerBoardSummary";

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
  const readyFeatures = featureWorkspace.features.filter(
    (feature) => feature.readiness === "ready",
  ).length;
  const blockedFeatures = featureWorkspace.features.filter(
    (feature) => feature.readiness === "blocked",
  ).length;
  const disabledFeatures = featureWorkspace.features.filter(
    (feature) => !feature.effective_enabled,
  ).length;
  const homeLoading = featureWorkspace.loading || boardSummaryLoading;

  async function handleRefreshHome() {
    await Promise.all([
      featureWorkspace.refresh(),
      refreshBoardSummary(),
    ]);
  }

  function renderPrimaryAction() {
    if (authState !== "signed_in") {
      return (
        <button
          className="button-primary"
          type="button"
          onClick={() => void beginLogin(nextPath)}
        >
          Sign in with Discord
        </button>
      );
    }

    if (selectedGuild === null) {
      return (
        <span className="meta-note">
          Choose a server from the sidebar to load server-scoped categories.
        </span>
      );
    }

    return (
      <Link className="button-primary" to={appRoutes.partnerBoardEntries}>
        Open Partner Board
      </Link>
    );
  }

  function renderSecondaryAction() {
    if (authState !== "signed_in" || selectedGuild === null) {
      return null;
    }

    return (
      <button
        className="button-secondary"
        type="button"
        disabled={homeLoading}
        onClick={() => void handleRefreshHome()}
      >
        Refresh home
      </button>
    );
  }

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
          ? "Choose a server to load category health and feature readiness."
          : "The selected server drives every server-scoped workspace.",
      tone: selectedGuild === null ? "info" : "neutral",
    },
    {
      label: "Feature areas",
      value:
        featureWorkspace.workspaceState === "ready"
          ? `${readyFeatures} ready`
          : formatHomeWorkspaceLabel(featureWorkspace.workspaceState),
      description:
        featureWorkspace.workspaceState === "ready"
          ? `${blockedFeatures} blocked • ${disabledFeatures} disabled`
          : formatHomeWorkspaceSupport(featureWorkspace.workspaceState),
      tone:
        blockedFeatures > 0
          ? "error"
          : featureWorkspace.workspaceState === "ready"
            ? "success"
            : "info",
    },
    {
      label: "Partner Board",
      value: shellStatus.label,
      description: shellStatus.description,
      tone: shellStatus.tone,
    },
  ] as const;

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Home"
        title="Home"
        description="Review server-scoped feature areas, open the main workspaces, and spot blocked categories before configuration work begins."
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
          <>
            {renderSecondaryAction()}
            {renderPrimaryAction()}
          </>
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

      <section className="home-area-grid" aria-label="Feature areas">
        <SurfaceCard className="home-area-card">
          <div className="home-area-card-header">
            <div className="card-copy">
              <p className="section-label">Workspace</p>
              <h2>Partner Board</h2>
              <p className="section-description">
                Entries, layout, and delivery stay together in the dedicated publishing workspace.
              </p>
            </div>
            <StatusBadge tone={shellStatus.tone}>{shellStatus.label}</StatusBadge>
          </div>

          <KeyValueList
            className="workspace-status-list"
            items={[
              {
                label: "Destination",
                value: summarizePostingDestination,
              },
              {
                label: "Posting method",
                value: postingMethodLabel,
              },
              {
                label: "Entries",
                value: String(partnerCount),
              },
              {
                label: "Layout",
                value: layoutConfigured ? "Ready" : "Needs setup",
              },
              {
                label: "Last checked",
                value: formatTimestamp(lastLoadedAt, "Not checked yet"),
              },
            ]}
          />

          <div className="home-area-footer">
            <Link className="button-primary" to={appRoutes.partnerBoardEntries}>
              Open Partner Board
            </Link>
            {!deliveryConfigured ? (
              <Link className="button-secondary" to={appRoutes.partnerBoardDelivery}>
                Finish destination
              </Link>
            ) : null}
          </div>
        </SurfaceCard>

        {featureAreaDefinitions.map((area) => {
          const areaFeatures = getFeatureAreaRecords(featureWorkspace.features, area.id);
          const areaSummary = summarizeFeatureArea(
            areaFeatures,
            featureWorkspace.workspaceState,
          );

          return (
            <SurfaceCard className="home-area-card" id={area.anchor} key={area.id}>
              <div className="home-area-card-header">
                <div className="card-copy">
                  <p className="section-label">Feature area</p>
                  <h2>{area.label}</h2>
                  <p className="section-description">{area.description}</p>
                </div>
                <StatusBadge tone={areaSummary.tone}>{areaSummary.label}</StatusBadge>
              </div>

              <ul className="home-area-list">
                <li className="home-area-row">
                  <span>Tracked features</span>
                  <strong>{areaSummary.total}</strong>
                </li>
                <li className="home-area-row">
                  <span>Ready</span>
                  <strong>{areaSummary.ready}</strong>
                </li>
                <li className="home-area-row">
                  <span>Blocked</span>
                  <strong>{areaSummary.blocked}</strong>
                </li>
                <li className="home-area-row">
                  <span>Disabled</span>
                  <strong>{areaSummary.disabled}</strong>
                </li>
                <li className="home-area-row">
                  <span>Current signal</span>
                  <strong>{areaSummary.support}</strong>
                </li>
              </ul>

              <div className="home-area-footer">
                <Link
                  className="button-secondary"
                  to={getFeatureAreaRoute(area.id)}
                >
                  Open {area.label}
                </Link>
              </div>
            </SurfaceCard>
          );
        })}

        <SurfaceCard className="home-area-card">
          <div className="home-area-card-header">
            <div className="card-copy">
              <p className="section-label">Technical</p>
              <h2>Settings</h2>
              <p className="section-description">
                Session state, control connection, and diagnostics stay separate from daily feature management.
              </p>
            </div>
            <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
              {formatAuthStateLabel(authState)}
            </StatusBadge>
          </div>

          <KeyValueList
            className="workspace-status-list"
            items={[
              {
                label: "Connection",
                value: currentOriginLabel,
              },
              {
                label: "Server access",
                value: formatAuthSupportText(authState, manageableGuilds.length),
              },
              {
                label: "Diagnostics",
                value: "Advanced connection and destination details stay there.",
              },
            ]}
          />

          <div className="home-area-footer">
            <Link className="button-primary" to={appRoutes.settings}>
              Open settings
            </Link>
          </div>
        </SurfaceCard>
      </section>

      <section className="home-planned-grid" id="planned" aria-label="Planned modules">
        {plannedModules.map((module) => (
          <SurfaceCard className="roadmap-card roadmap-card-muted home-planned-card" key={module.id}>
            <div className="card-copy">
              <p className="section-label">Planned module</p>
              <h2>{module.label}</h2>
              <p className="section-description">{module.description}</p>
            </div>
            <div className="home-area-footer">
              <span className="meta-note">
                This stays off the main navigation until the operator workflow is intentionally designed.
              </span>
            </div>
          </SurfaceCard>
        ))}
      </section>
    </section>
  );
}

function formatHomeWorkspaceLabel(
  state: ReturnType<typeof useFeatureWorkspace>["workspaceState"],
) {
  switch (state) {
    case "checking":
      return "Checking access";
    case "auth_required":
      return "Sign in required";
    case "server_required":
      return "Choose a server";
    case "loading":
      return "Loading categories";
    case "unavailable":
      return "Unavailable";
    case "ready":
      return "Ready";
    default:
      return "Unavailable";
  }
}

function formatHomeWorkspaceSupport(
  state: ReturnType<typeof useFeatureWorkspace>["workspaceState"],
) {
  switch (state) {
    case "checking":
      return "The dashboard is verifying session access.";
    case "auth_required":
      return "Sign in with Discord to load feature categories.";
    case "server_required":
      return "Choose a server to load guild feature readiness.";
    case "loading":
      return "Loading the latest feature records for the selected server.";
    case "unavailable":
      return "The feature workspace could not be loaded for this server.";
    case "ready":
      return "Feature categories are loaded and ready to summarize.";
    default:
      return "Feature categories are unavailable.";
  }
}

function summarizeFeatureArea(
  features: ReturnType<typeof getFeatureAreaRecords>,
  workspaceState: ReturnType<typeof useFeatureWorkspace>["workspaceState"],
) {
  if (workspaceState !== "ready") {
    return {
      blocked: 0,
      disabled: 0,
      label: formatHomeWorkspaceLabel(workspaceState),
      ready: 0,
      support: formatHomeWorkspaceSupport(workspaceState),
      tone: "info" as const,
      total: 0,
    };
  }

  const total = features.length;
  const ready = features.filter((feature) => feature.readiness === "ready").length;
  const blocked = features.filter((feature) => feature.readiness === "blocked").length;
  const disabled = features.filter((feature) => !feature.effective_enabled).length;

  let tone: "neutral" | "info" | "success" | "error" = "neutral";
  let label = "Disabled";
  let support = total === 0 ? "No feature records are mapped to this category yet." : "Everything is currently disabled.";

  if (blocked > 0) {
    tone = "error";
    label = "Needs attention";
    support =
      features.find((feature) => feature.blockers?.length)?.blockers?.[0]?.message ??
      "One or more features in this category are blocked.";
  } else if (ready > 0) {
    tone = "success";
    label = "Operational";
    support =
      disabled === 0
        ? "Every mapped feature in this area is ready."
        : "At least one feature in this area is ready to use.";
  } else if (total > 0) {
    tone = "neutral";
    label = "Disabled";
  }

  return {
    blocked,
    disabled,
    label,
    ready,
    support,
    tone,
    total,
  };
}
