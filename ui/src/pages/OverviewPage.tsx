import { Link, useLocation } from "react-router-dom";
import { appRoutes } from "../app/routes";
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
import { usePartnerBoardSummary } from "../features/partner-board/usePartnerBoardSummary";

export function OverviewPage() {
  const location = useLocation();
  const {
    authState,
    beginLogin,
    manageableGuilds,
    selectedGuild,
  } = useDashboardSession();
  const {
    deliveryConfigured,
    lastLoadedAt,
    layoutConfigured,
    loading,
    partnerCount,
    postingMethodLabel,
    refreshBoardSummary,
    shellStatus,
    summarizePostingDestination,
  } = usePartnerBoardSummary();

  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const lastCheckedLabel = formatTimestamp(lastLoadedAt, "Not checked yet");
  const selectedServerLabel = selectedGuild?.name ?? "No server selected";

  let nextActionTitle = "Connect your dashboard session";
  let nextActionDescription =
    "Sign in with Discord before loading any server-scoped dashboard workspace.";

  if (authState === "signed_in" && selectedGuild === null) {
    nextActionTitle = "Choose the active server";
    nextActionDescription =
      "Server scope is global in the dashboard. Select one server to load the operational workspace.";
  } else if (authState === "signed_in" && !deliveryConfigured) {
    nextActionTitle = "Finish the posting destination";
    nextActionDescription =
      "Define where the board publishes before relying on the workspace for steady-state management.";
  } else if (authState === "signed_in" && partnerCount === 0) {
    nextActionTitle = "Add the first partner entry";
    nextActionDescription =
      "The board shell is ready. Seed at least one partner so layout and publishing can be verified with real data.";
  } else if (authState === "signed_in" && !layoutConfigured) {
    nextActionTitle = "Finalize the board layout";
    nextActionDescription =
      "Fill the core copy fields so the board is publishable without exposing advanced formatting in the default flow.";
  } else if (authState === "signed_in" && selectedGuild !== null) {
    nextActionTitle = "Partner Board is ready to manage";
    nextActionDescription =
      "Entries, layout, and delivery are configured enough for day-to-day operations from the feature workspace.";
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
      return <span className="meta-note">Choose a server from the top bar to continue.</span>;
    }

    if (!deliveryConfigured) {
      return (
        <Link className="button-primary" to={appRoutes.partnerBoardDelivery}>
          Finish destination
        </Link>
      );
    }

    if (partnerCount === 0) {
      return (
        <Link className="button-primary" to={appRoutes.partnerBoardEntries}>
          Add first partner
        </Link>
      );
    }

    if (!layoutConfigured) {
      return (
        <Link className="button-primary" to={appRoutes.partnerBoardLayout}>
          Edit layout
        </Link>
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
        disabled={loading}
        onClick={() => void refreshBoardSummary()}
      >
        Refresh status
      </button>
    );
  }

  function renderNextStepAction() {
    if (authState !== "signed_in") {
      return (
        <button
          className="button-secondary"
          type="button"
          onClick={() => void beginLogin(nextPath)}
        >
          Connect session
        </button>
      );
    }

    if (selectedGuild === null) {
      return (
        <span className="meta-note">
          Choose a server from the top bar to continue.
        </span>
      );
    }

    if (!deliveryConfigured) {
      return (
        <Link className="button-secondary" to={appRoutes.partnerBoardDelivery}>
          Open destination setup
        </Link>
      );
    }

    if (partnerCount === 0) {
      return (
        <Link className="button-secondary" to={appRoutes.partnerBoardEntries}>
          Open entry manager
        </Link>
      );
    }

    if (!layoutConfigured) {
      return (
        <Link className="button-secondary" to={appRoutes.partnerBoardLayout}>
          Open layout editor
        </Link>
      );
    }

    return (
      <Link className="button-secondary" to={appRoutes.partnerBoardEntries}>
        Open workspace
      </Link>
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
      label: "Server scope",
      value: selectedServerLabel,
      description:
        selectedGuild === null
          ? "Select a server in the top bar to load its workspace."
          : "The current scope applies across dashboard pages.",
      tone: selectedGuild === null ? "info" : "neutral",
    },
    {
      label: "Partner Board",
      value: shellStatus.label,
      description: shellStatus.description,
      tone: shellStatus.tone,
    },
    {
      label: "Entries",
      value: String(partnerCount),
      description:
        selectedGuild === null
          ? "Counts appear after a server is selected."
          : `${postingMethodLabel} • ${summarizePostingDestination}`,
      tone: partnerCount > 0 ? "success" : "neutral",
    },
  ] as const;

  const overviewStatusItems = [
    {
      label: "Setup state",
      value: <StatusBadge tone={shellStatus.tone}>{shellStatus.label}</StatusBadge>,
    },
    {
      label: "Posting destination",
      value: summarizePostingDestination,
    },
    {
      label: "Posting method",
      value: postingMethodLabel,
    },
    {
      label: "Partner entries",
      value: String(partnerCount),
    },
    {
      label: "Last checked",
      value: lastCheckedLabel,
    },
  ];

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Overview"
        title="Overview"
        description="Check access, server readiness, and Partner Board setup for the selected server."
        status={
          <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
            {formatAuthStateLabel(authState)}
          </StatusBadge>
        }
        meta={
          <>
            <span className="meta-pill subtle-pill">{selectedServerLabel}</span>
            <span className="meta-pill subtle-pill">Last checked {lastCheckedLabel}</span>
          </>
        }
        actions={
          <>
            {renderSecondaryAction()}
            {renderPrimaryAction()}
          </>
        }
      />

      <section className="overview-summary-strip" aria-label="Server status">
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

      <div className="overview-main-grid">
        <SurfaceCard className="feature-status-card overview-primary-card">
          <div className="overview-card-header">
            <div className="card-copy">
              <p className="section-label">Feature status</p>
              <h2>Partner Board</h2>
              <p className="section-description">
                Manage entries, layout, and publishing for the selected server.
              </p>
            </div>
            <StatusBadge tone={shellStatus.tone}>{shellStatus.label}</StatusBadge>
          </div>

          <KeyValueList className="overview-status-list" items={overviewStatusItems} />

          <section className="overview-next-step">
            <div className="overview-next-step-copy">
              <p className="section-label">Next step</p>
              <strong>{nextActionTitle}</strong>
              <p className="section-description">{nextActionDescription}</p>
            </div>
            <div className="overview-next-step-actions">{renderNextStepAction()}</div>
          </section>
        </SurfaceCard>

        <SurfaceCard className="roadmap-card roadmap-card-muted" id="roadmap">
          <div className="card-copy">
            <p className="section-label">Roadmap</p>
            <h2>Planned areas</h2>
            <p className="section-description">
              Future areas stay visible for planning, but remain secondary until they become operational workspaces.
            </p>
          </div>

          <div className="roadmap-list overview-roadmap-list" aria-label="Planned feature roadmap">
            <article className="roadmap-item">
              <strong>Moderation</strong>
              <p>Rules, reports, and enforcement workflows will arrive once the first real tools are ready.</p>
            </article>
            <article className="roadmap-item">
              <strong>Automations</strong>
              <p>Scheduled workflows stay hidden until the dashboard can run and inspect real automations.</p>
            </article>
            <article className="roadmap-item">
              <strong>Global activity</strong>
              <p>Cross-feature history returns only after feature-level events exist and can be filtered meaningfully.</p>
            </article>
          </div>
        </SurfaceCard>
      </div>
    </section>
  );
}
